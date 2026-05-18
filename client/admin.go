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
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

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
	Title    string
	Theme    string
	Owner    string
	Sections []AdminSection
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
		{ID: "logging", Title: "Logging", Icon: "📜", Method: "grain_updatelogging", Config: cfg.Logging},
		{ID: "auth", Title: "Auth", Icon: "🔐", Method: "grain_updateauth", Config: cfg.Auth},
		{ID: "event_purge", Title: "Event purge", Icon: "🧹", Method: "grain_updateeventpurge", Config: cfg.EventPurge},
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
		Title:    "🌾 grain — admin",
		Owner:    owner,
		Sections: sections,
	}
	renderAdmin(w, data)
}

// renderAdmin parses the admin template against the shared layout and
// renders it. Mirrors RenderTemplate but with a typed data argument
// (PageData is too narrow — admin needs Sections + Owner).
func renderAdmin(w http.ResponseWriter, data AdminPageData) {
	viewTemplate := path.Join(viewsDir, "admin.html")
	componentPattern := path.Join(viewsDir, "components", "*.html")
	componentTemplates, err := fs.Glob(wwwFS, componentPattern)
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	patterns := append(layoutPatterns(), viewTemplate)
	patterns = append(patterns, componentTemplates...)
	tmpl, err := template.New("").Funcs(template.FuncMap{}).ParseFS(wwwFS, patterns...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}
