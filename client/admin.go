// Admin dashboard: server-rendered page at /admin for the relay owner
// to tune every config knob live via NIP-86.
//
// The gate here is the cookie session, not NIP-98 — NIP-98 is what the
// dispatcher uses for the per-action grain_* writes the page issues
// from the browser. We render the shell only if the cookie-session
// pubkey matches the relay_metadata.json owner; non-owners get a 303
// to "/" with no content leak.
//
// Lives in the client package (not server/api) because rendering goes
// through RenderTemplate, which is defined here. Putting the handler
// in server/api would create an import cycle.
package client

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// adminTemplateFuncs are the template helpers admin pages need.
// Lives here (not in templateEngine.go) so it doesn't bleed into
// every page render — admin's the only page using these today.
var adminTemplateFuncs = template.FuncMap{
	// assetVersion stamps static asset URLs in the shared layout;
	// admin renders that layout too, so it must register the func.
	"assetVersion": func() string { return assetVersion },
	// toJS marshals any value to JSON and returns it as
	// template.JS so the renderer doesn't HTML-escape the
	// resulting literal. Safe for inline <script> use because the
	// input here is a small, static map we control.
	"toJS": func(v any) template.JS {
		b, err := json.Marshal(v)
		if err != nil {
			return template.JS("null")
		}
		return template.JS(b)
	},
	// rateLimitJSON returns JSON as a plain string. Used inside
	// HTML attributes where html/template's auto-escaping takes
	// care of quoting. (toJS would bypass escaping — wrong choice
	// for attribute context.)
	"rateLimitJSON": func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		return string(b)
	},
	// kindLabelStr resolves a stringified kind ("30023") to its
	// human label via the curated KindLabels map. Empty string
	// when the kind isn't catalogued — caller can decide whether
	// to suppress the label or show "(no description)".
	"kindLabelStr": func(s string) string {
		k, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return ""
		}
		return KindLabels[k]
	},
}

// LoggingSectionData is the per-section template data for the
// logging form. We can't render the suppress-components UI from
// just LogConfig — the form needs the full set of known component
// names (so an operator gets checkboxes instead of typing names
// they have to guess), and that catalog lives in
// server/utils/log/components.go. Bundling the two here keeps the
// template clean and the catalog discoverable.
type LoggingSectionData struct {
	Config        cfgType.LogConfig
	AllComponents []string
}

// EventPurgeSectionData is the per-section template data for the
// event_purge form. The form renders one checkbox per known purge
// category (the v0.4-compat names from
// server/db/nostrdb/purge.go:purgeCategoryForKind); rather than
// teach the template to construct a literal slice, we hand it the
// list directly. CommonKinds drives the quick-add chip row above
// the kinds_to_purge textarea.
type EventPurgeSectionData struct {
	Config      cfgType.EventPurgeConfig
	Categories  []string
	CommonKinds []QuickKind
	// KindLabels duplicated here from the page-level data because
	// Go templates lose access to the outer dot once a sub-template
	// is invoked. Cheap to pass — it's a single map reference.
	KindLabels map[int]string
}

// QuickKind is one entry in the kinds_to_purge quick-add chip row.
type QuickKind struct {
	Kind  int
	Label string
}

// commonPurgeKinds is the suggested-purge starter set: high-volume
// kinds that operators most often want to evict. Curated rather
// than exhaustive — chips are an affordance, not a catalog. The
// textarea accepts any non-negative integer.
var commonPurgeKinds = []QuickKind{
	{Kind: 7, Label: "reactions"},
	{Kind: 6, Label: "reposts"},
	{Kind: 9735, Label: "zap receipts"},
	{Kind: 1059, Label: "gift-wrap (NIP-17)"},
	{Kind: 16, Label: "generic repost"},
}

// RateLimitSectionData is the per-section template data for the
// rate_limit form. Carries the typed config plus reference
// catalogs the form needs (category list for deterministic order,
// suggested per-kind size and rate presets for the quick-add
// chip rows).
type RateLimitSectionData struct {
	Config              cfgType.RateLimitConfig
	RateLimitCategories []string
	CategoryDefaults    map[string]map[string]float64 // category → {limit, burst}
	KindSizePresets     []KindSizePreset
	KindRatePresets     []KindRatePreset
}

// KindSizePreset is one suggested kind→max_size pairing on the
// per-kind size limits quick-add row. Sizes here are operator-
// friendly defaults — operators can edit before clicking Add.
type KindSizePreset struct {
	Kind  int
	Bytes int
}

// KindRatePreset is one suggested kind→{limit, burst} on the
// per-kind rate quick-add row.
type KindRatePreset struct {
	Kind  int
	Limit float64
	Burst int
}

// categoryDefaultsForRateLimit is what we pre-populate when an
// operator flips a category's Enable toggle ON for a category that
// has no entry on disk yet. Matches the rule-of-thumb numbers in
// the example config.
var categoryDefaultsForRateLimit = map[string]map[string]float64{
	"regular":     {"limit": 8, "burst": 16},
	"replaceable": {"limit": 5, "burst": 10},
	"ephemeral":   {"limit": 50, "burst": 100},
	"addressable": {"limit": 3, "burst": 8},
}

// kindSizePresets / kindRatePresets are the chips operators click
// to pre-fill the add-row inputs. Sizes/rates here are
// rule-of-thumb starting points, not policy — operators tweak
// before clicking Add.
var kindSizePresets = []KindSizePreset{
	{Kind: 0, Bytes: 8 * 1024},       // User Metadata
	{Kind: 1, Bytes: 4 * 1024},       // Short Text Note
	{Kind: 3, Bytes: 64 * 1024},      // Follow List
	{Kind: 7, Bytes: 512},            // Reactions
	{Kind: 9735, Bytes: 4 * 1024},    // Zap Receipts
	{Kind: 30023, Bytes: 256 * 1024}, // Long-form Content
}

var kindRatePresets = []KindRatePreset{
	{Kind: 0, Limit: 1, Burst: 2},      // Profile metadata — tight
	{Kind: 1, Limit: 5, Burst: 12},     // Notes — tighter than category
	{Kind: 6, Limit: 5, Burst: 12},     // Reposts
	{Kind: 7, Limit: 20, Burst: 40},    // Reactions — loose, tiny
	{Kind: 9735, Limit: 10, Burst: 20}, // Zap receipts
	{Kind: 30023, Limit: 1, Burst: 3},  // Long-form — slow authored
}

// OpsSectionData is the per-section template data for ops. The
// dashboard renders the current relay identity (name/description/
// icon/banner/contact + policy URLs) so an operator can edit them
// in place, plus a stats area fetched live via grain_stats_overview.
// Cache refresh and config reload run via separate buttons.
type OpsSectionData struct {
	RelayName           string
	RelayDescription    string
	RelayIcon           string
	RelayBanner         string
	RelayContact        string
	RelayPrivacyPolicy  string
	RelayTermsOfService string
	RelayPostingPolicy  string
}

// BlacklistSectionData wraps BlacklistConfig with the same
// unified pubkey treatment the whitelist gets: hex + npub merge
// into one display list, mutelist authors render with profile
// previews. IP scalars ride through the bulk save now that
// UpdateBlacklistConfig writes them to config.yml. The IP LIST
// is edited live (per-row blockip/unblockip) rather than via the
// section Save, so it has no snapshot-replay path.
type BlacklistSectionData struct {
	Config                cfgType.BlacklistConfig
	UnifiedPubkeys        []UnifiedPubkey
	BrokenPubkeys         []string // bad entries from blacklist.yml's pubkeys/npubs
	MutelistAuthors       []UnifiedPubkey
	BrokenMutelistAuthors []string

	// Owner's private mute list sync status (#60). Rendered so the owner
	// knows when they last synced and how many pubkeys it contributed.
	// Zero values mean "never synced".
	OwnerMutelistCount  int
	OwnerMutelistSynced int64 // unix seconds; 0 = never

	// The owner's synced private-mute pubkeys, for in-panel display with
	// profiles. Owner-only page, so showing the actual keys is fine here
	// (the public endpoint stays count-only).
	OwnerMutelistPubkeys []UnifiedPubkey
}

// WhitelistSectionData wraps WhitelistConfig with a unified
// pubkey view + the kind catalog needed by the form. The
// dashboard renders one row per UnifiedPubkey showing both hex
// and npub regardless of which form the operator originally
// entered; the wire shape (Pubkeys vs Npubs) is collapsed on
// save into Pubkeys-only by the JS submit path.
type WhitelistSectionData struct {
	Config         cfgType.WhitelistConfig
	UnifiedPubkeys []UnifiedPubkey
	BrokenPubkeys  []string // raw entries from the YAML that didn't parse
	KindLabels     map[int]string
	KindPresets    []QuickKind
}

// UnifiedPubkey is one row of the merged hex+npub display. Both
// forms are precomputed server-side so the page doesn't have to
// fan out N convert-API calls on first render.
type UnifiedPubkey struct {
	Hex  string
	Npub string
}

// whitelistKindPresets are the chips on the kind-whitelist input.
// Common scenarios:
//   - indexing relay: 0, 3, 10002
//   - general public relay: 1, 7, 30023
//   - app-specific: operator types their custom kind in the int input
var whitelistKindPresets = []QuickKind{
	{Kind: 0, Label: "metadata"},
	{Kind: 1, Label: "notes"},
	{Kind: 3, Label: "follow list"},
	{Kind: 7, Label: "reactions"},
	{Kind: 10002, Label: "relay list"},
	{Kind: 30023, Label: "long-form"},
}

// buildUnifiedPubkeys merges the wire shape's hex Pubkeys + bech32
// Npubs into a single deduplicated slice. Entries that fail to
// parse (bad hex length, undecodable npub) get surfaced separately
// via the second return so the dashboard can warn the operator
// rather than silently dropping them. Saving through the dashboard
// always drops the broken ones from the file — but the operator
// at least knows what was there.
func buildUnifiedPubkeys(hexes, npubs []string) ([]UnifiedPubkey, []string) {
	seen := make(map[string]bool, len(hexes)+len(npubs))
	out := make([]UnifiedPubkey, 0, len(hexes)+len(npubs))
	broken := make([]string, 0)
	for _, h := range hexes {
		raw := h
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if len(h) != 64 || !isLowerHexAll(h) {
			broken = append(broken, raw)
			continue
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		np, _ := tools.EncodePubkey(h)
		out = append(out, UnifiedPubkey{Hex: h, Npub: np})
	}
	for _, n := range npubs {
		raw := n
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		hex, err := tools.DecodeNpub(n)
		if err != nil || hex == "" {
			broken = append(broken, raw)
			continue
		}
		if seen[hex] {
			continue
		}
		seen[hex] = true
		out = append(out, UnifiedPubkey{Hex: hex, Npub: n})
	}
	return out, broken
}

// isLowerHexAll is duplicated from setup.go's identical helper to
// avoid cross-package import; whitelist-side hex validation runs
// on operator-supplied YAML data so we want our own copy.
func isLowerHexAll(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// rateLimitCategories is the fixed display order for the
// per-category rates form. Matches the four named keys in the
// example config + the upstream category enums in
// server/db/nostrdb/purge.go:purgeCategoryForKind. The "unknown"
// and "deprecated" categories don't apply here because per-event
// rate-limiting buckets unknown kinds with "regular".
var rateLimitCategories = []string{
	"regular",
	"replaceable",
	"addressable",
	"ephemeral",
}

// purgeCategories is the subset of purgeCategoryForKind's enum
// that the form exposes. "deprecated" (kind 2) and "ephemeral"
// (20000-29999) are dropped at ingest in store.go and never reach
// the purger, so toggling them in the form would have no effect.
// "unknown" stays because those kinds (gaps in NIP-01 ranges +
// 40000+ experimental) ARE stored as regular and an operator may
// want to purge them separately. Keep this list and
// purgeCategoryForKind in sync as new ranges are added.
// purgeCategories is the baseline set of category checkboxes the event-purge
// form always shows: the categories that are actually stored and that an
// operator meaningfully tunes. purgeCategoriesFor unions in any other keys the
// config carries (a legacy "deprecated", or "unknown" for 40000+ kinds) so the
// form mirrors the dashboard, which renders the same config map. Deprecated
// (kind 2) and ephemeral (20000–29999) events are never stored (see nostrdb
// store.go), so those categories surface only when a config explicitly lists
// them and purging them is a no-op. "addressable" is the current name for the
// 30000–39999 range (the purger still aliases the old
// "parameterized_replaceable").
var purgeCategories = []string{
	"regular",
	"replaceable",
	"addressable",
}

// purgeCategoriesFor returns the categories the event-purge form renders: the
// canonical set above plus any extra keys the operator's config happens to
// carry. Seeding from the live config map (not just the static list) means the
// form always mirrors what the dashboard shows from that same map — no
// configured category can silently go missing from the editor.
func purgeCategoriesFor(configured map[string]bool) []string {
	seen := make(map[string]bool, len(purgeCategories))
	for _, c := range purgeCategories {
		seen[c] = true
	}
	extra := make([]string, 0, len(configured))
	for c := range configured {
		if !seen[c] {
			extra = append(extra, c)
		}
	}
	sort.Strings(extra) // stable order — map iteration is randomized
	return append(append([]string(nil), purgeCategories...), extra...)
}

// AdminSection is one panel in the accordion. Config is the typed
// config blob (or nil for ops) — the template renders it inside the
// stub <pre> in Phase 1, and Phase 2+ commits read individual fields
// off the typed struct as they replace each stub with a real form.
type AdminSection struct {
	ID     string
	Title  string
	Icon   string
	Method string // grain_* write method this section targets (empty for ops)
	Config any
}

// AdminPageData is what admin.html renders against.
type AdminPageData struct {
	Title      string
	Theme      string
	Owner      string
	Sections   []AdminSection
	KindLabels map[int]string
}

// HandleAdmin renders the dashboard for the relay owner only.
//
// Gate: session cookie -> SessionMgr.GetCurrentUser -> compare to
// GetRelayOwnerPubkey (case-insensitive). Non-owner / no session ->
// 303 redirect to "/".
func HandleAdmin(w http.ResponseWriter, r *http.Request) {
	user := session.SessionMgr.GetCurrentUser(r)
	owner := utils.GetRelayOwnerPubkey()
	if user == nil || utils.IsRelayUnowned() || !strings.EqualFold(user.PublicKey, owner) {
		log.ClientAPI().Info("Admin page access denied",
			"client_ip", utils.GetClientIP(r),
			"has_session", user != nil)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	cfg := config.GetConfig()
	if cfg == nil {
		http.Error(w, "Server configuration not available", http.StatusInternalServerError)
		return
	}
	wl := config.GetWhitelistConfig()

	sections := []AdminSection{
		{ID: "ops", Title: "Operations", Icon: "🛠️", Method: "", Config: nil},
		{ID: "relay_info", Title: "Relay information", Icon: "🪪", Method: "",
			Config: func() OpsSectionData {
				m := utils.GetRelayMetadataCopy()
				return OpsSectionData{
					RelayName:           m.Name,
					RelayDescription:    m.Description,
					RelayIcon:           m.Icon,
					RelayBanner:         m.Banner,
					RelayContact:        m.Contact,
					RelayPrivacyPolicy:  m.PrivacyPolicy,
					RelayTermsOfService: m.TermsOfService,
					RelayPostingPolicy:  m.PostingPolicy,
				}
			}()},
		{ID: "logging", Title: "Logging", Icon: "📜", Method: "grain_updatelogging",
			Config: LoggingSectionData{Config: cfg.Logging, AllComponents: log.GetAllComponents()}},
		{ID: "auth", Title: "Auth", Icon: "🔐", Method: "grain_updateauth", Config: cfg.Auth},
		{ID: "event_purge", Title: "Event purge", Icon: "🧹", Method: "grain_updateeventpurge",
			Config: EventPurgeSectionData{
				Config:      cfg.EventPurge,
				Categories:  purgeCategoriesFor(cfg.EventPurge.PurgeByCategory),
				CommonKinds: commonPurgeKinds,
				KindLabels:  KindLabels,
			}},
		{ID: "event_time_constraints", Title: "Event time constraints", Icon: "⏱️", Method: "grain_updateeventtimeconstraints", Config: cfg.EventTimeConstraints},
		{ID: "backup_relay", Title: "Backup relay", Icon: "🪞", Method: "grain_updatebackuprelay", Config: cfg.BackupRelay},
		{ID: "client", Title: "Client", Icon: "🧩", Method: "grain_updateclient", Config: cfg.Client},
		{ID: "rate_limit", Title: "Rate limit", Icon: "🚦", Method: "grain_updateratelimit",
			Config: RateLimitSectionData{
				Config:              cfg.RateLimit,
				RateLimitCategories: rateLimitCategories,
				CategoryDefaults:    categoryDefaultsForRateLimit,
				KindSizePresets:     kindSizePresets,
				KindRatePresets:     kindRatePresets,
			}},
		{ID: "resource_limits", Title: "Resource limits", Icon: "📦", Method: "grain_updateresourcelimits", Config: cfg.ResourceLimits},
		{ID: "server", Title: "Server", Icon: "🖥️", Method: "grain_updateserver", Config: cfg.Server},
		{ID: "whitelist", Title: "Whitelist", Icon: "✅", Method: "grain_updatewhitelistconfig",
			Config: func() WhitelistSectionData {
				unified, broken := buildUnifiedPubkeys(wl.PubkeyWhitelist.Pubkeys, wl.PubkeyWhitelist.Npubs)
				return WhitelistSectionData{
					Config:         *wl,
					UnifiedPubkeys: unified,
					BrokenPubkeys:  broken,
					KindLabels:     KindLabels,
					KindPresets:    whitelistKindPresets,
				}
			}()},
		{ID: "blacklist", Title: "Blacklist", Icon: "⛔", Method: "grain_updateblacklistconfig",
			Config: func() BlacklistSectionData {
				// Read from blacklist.yml (the authoritative source for
				// pubkeys/npubs/words/mutelist_authors), not cfg.Blacklist —
				// config.yml's blacklist: section only carries the IP scalars,
				// so reading the list fields from there renders them empty even
				// when blacklist.yml holds entries (#85). Overlay the config.yml
				// IP scalars on top, mirroring runGetBlacklistConfig's merge.
				bl := cfg.Blacklist
				if standalone := config.GetBlacklistConfig(); standalone != nil {
					bl = *standalone
					bl.PermanentBlockedIPs = cfg.Blacklist.PermanentBlockedIPs
					bl.IPMaxTempBans = cfg.Blacklist.IPMaxTempBans
					bl.IPTempBanDuration = cfg.Blacklist.IPTempBanDuration
					bl.IPRateViolationThreshold = cfg.Blacklist.IPRateViolationThreshold
				}
				unified, broken := buildUnifiedPubkeys(bl.PermanentBlacklistPubkeys, bl.PermanentBlacklistNpubs)
				mute, brokenMute := buildUnifiedPubkeys(bl.MuteListAuthors, nil)
				// Owner's private mutelist sync status (#60), if they've synced.
				var ownerCount int
				var ownerSynced int64
				for _, m := range config.GetAdminMutelistMeta() {
					if strings.EqualFold(m.Pubkey, owner) {
						ownerCount = m.Count
						ownerSynced = m.SyncedAt
						break
					}
				}
				ownerMutes, _ := buildUnifiedPubkeys(config.GetAdminMutelistPubkeysFor(owner), nil)
				return BlacklistSectionData{
					Config:                bl,
					UnifiedPubkeys:        unified,
					BrokenPubkeys:         broken,
					MutelistAuthors:       mute,
					BrokenMutelistAuthors: brokenMute,
					OwnerMutelistCount:    ownerCount,
					OwnerMutelistSynced:   ownerSynced,
					OwnerMutelistPubkeys:  ownerMutes,
				}
			}()},
	}

	// Operator-facing display order: the highest-traffic sections first; any not
	// listed keep their definition order behind them.
	sections = orderSections(sections, "ops", "relay_info", "server", "rate_limit", "resource_limits", "whitelist", "blacklist")

	data := AdminPageData{
		Title:      "🌾 grain — admin",
		Owner:      owner,
		Sections:   sections,
		KindLabels: KindLabels,
	}
	renderAdmin(w, data)
}

// orderSections returns sections with the given IDs first, in the order given;
// any section not named keeps its original position behind them. Lets the
// definition block stay grouped by concern while the display order is curated.
func orderSections(sections []AdminSection, firstIDs ...string) []AdminSection {
	listed := make(map[string]bool, len(firstIDs))
	for _, id := range firstIDs {
		listed[id] = true
	}
	out := make([]AdminSection, 0, len(sections))
	for _, id := range firstIDs {
		for _, s := range sections {
			if s.ID == id {
				out = append(out, s)
				break
			}
		}
	}
	for _, s := range sections {
		if !listed[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// renderAdmin parses the admin template against the shared layout and
// renders it. Mirrors RenderTemplate but with a typed data argument
// (PageData is too narrow — admin needs Sections + Owner).
//
// Per-section partials live under www/views/admin-sections/*.html and
// each defines a template named after its section (e.g. "admin-logging"
// is invoked from admin.html with {{template "admin-logging" .Config}}).
// Sections without a partial yet fall back to the JSON-pretty-print stub.
func renderAdmin(w http.ResponseWriter, data AdminPageData) {
	viewTemplate := path.Join(viewsDir, "admin.html")
	componentTemplates, err := fs.Glob(wwwFS, path.Join(viewsDir, "components", "*.html"))
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	sectionTemplates, err := fs.Glob(wwwFS, path.Join(viewsDir, "admin-sections", "*.html"))
	if err != nil {
		http.Error(w, "Error loading admin-section templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	patterns := append(layoutPatterns(), viewTemplate)
	patterns = append(patterns, componentTemplates...)
	patterns = append(patterns, sectionTemplates...)
	tmpl, err := template.New("").Funcs(adminTemplateFuncs).ParseFS(wwwFS, patterns...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}
