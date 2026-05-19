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
	"strings"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// adminTemplateFuncs are the template helpers admin pages need.
// Lives here (not in templateEngine.go) so it doesn't bleed into
// every page render — admin's the only page using toJS today.
var adminTemplateFuncs = template.FuncMap{
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

// purgeCategories is the subset of purgeCategoryForKind's enum
// that the form exposes. "deprecated" (kind 2) and "ephemeral"
// (20000-29999) are dropped at ingest in store.go and never reach
// the purger, so toggling them in the form would have no effect.
// "unknown" stays because those kinds (gaps in NIP-01 ranges +
// 40000+ experimental) ARE stored as regular and an operator may
// want to purge them separately. Keep this list and
// purgeCategoryForKind in sync as new ranges are added.
var purgeCategories = []string{
	"replaceable",
	"regular",
	"parameterized_replaceable",
	"unknown",
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
		{ID: "logging", Title: "Logging", Icon: "📜", Method: "grain_updatelogging",
			Config: LoggingSectionData{Config: cfg.Logging, AllComponents: log.GetAllComponents()}},
		{ID: "auth", Title: "Auth", Icon: "🔐", Method: "grain_updateauth", Config: cfg.Auth},
		{ID: "event_purge", Title: "Event purge", Icon: "🧹", Method: "grain_updateeventpurge",
			Config: EventPurgeSectionData{
				Config:      cfg.EventPurge,
				Categories:  purgeCategories,
				CommonKinds: commonPurgeKinds,
				KindLabels:  KindLabels,
			}},
		{ID: "event_time_constraints", Title: "Event time constraints", Icon: "⏱️", Method: "grain_updateeventtimeconstraints", Config: cfg.EventTimeConstraints},
		{ID: "backup_relay", Title: "Backup relay", Icon: "🪞", Method: "grain_updatebackuprelay", Config: cfg.BackupRelay},
		{ID: "rate_limit", Title: "Rate limit", Icon: "🚦", Method: "grain_updateratelimit", Config: cfg.RateLimit},
		{ID: "resource_limits", Title: "Resource limits", Icon: "📦", Method: "grain_updateresourcelimits", Config: cfg.ResourceLimits},
		{ID: "server", Title: "Server", Icon: "🖥️", Method: "grain_updateserver", Config: cfg.Server},
		{ID: "whitelist", Title: "Whitelist", Icon: "✅", Method: "grain_updatewhitelistconfig", Config: wl},
		{ID: "blacklist", Title: "Blacklist", Icon: "⛔", Method: "grain_updateblacklistconfig", Config: cfg.Blacklist},
		{ID: "ops", Title: "Operations", Icon: "🛠️", Method: "", Config: nil},
	}

	data := AdminPageData{
		Title:      "🌾 grain — admin",
		Owner:      owner,
		Sections:   sections,
		KindLabels: KindLabels,
	}
	renderAdmin(w, data)
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
