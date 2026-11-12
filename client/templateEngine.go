package client

import (
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type PageData struct {
	Title string
	Theme string
}

// defaultBrandName is grain's own name, shown in the browser tab and the
// header when the relay has no name configured. Per-relay branding rides in
// through the relay metadata (relay_metadata.json, editable from /admin; see
// SetRelayBranding) and is never baked in here.
const defaultBrandName = "🌾 grain"

// RelayBranding is what grain's own web pages show of the relay's identity:
// Name (NIP-11 name: tab title, header text when there is no logo, logo alt
// text), Icon (NIP-11 icon: the tab favicon and the PWA icon), Logo (a
// grain-only, non-NIP-11 setting: the header image, e.g. a wordmark) and
// LogoLight (grain-only, optional: the header image under light themes;
// only used with a Logo).
type RelayBranding struct {
	Name      string
	Icon      string
	Logo      string
	LogoLight string
}

// relayBranding, when wired by startup via SetRelayBranding, returns the
// relay's configured branding from the live relay metadata. Kept as a
// provider func rather than a snapshot so a NIP-86 changerelayname /
// changerelayicon / grain_changerelaylogo[light] that mutates the in-memory
// metadata is reflected on the very next page render.
var relayBranding func() RelayBranding

// SetRelayBranding wires the relay's branding source so the shared
// layout/header and the PWA manifest can render it. Called once at startup
// after relay metadata is loaded.
func SetRelayBranding(fn func() RelayBranding) { relayBranding = fn }

// currentBranding is the trimmed branding for one render; zero when unwired.
func currentBranding() RelayBranding {
	if relayBranding == nil {
		return RelayBranding{}
	}
	b := relayBranding()
	return RelayBranding{
		Name:      strings.TrimSpace(b.Name),
		Icon:      webImageURL(b.Icon),
		Logo:      webImageURL(b.Logo),
		LogoLight: webImageURL(b.LogoLight),
	}
}

// webImageURL is v when a page can load it as an image: an absolute http(s)
// URL or a plain path on this relay. Anything else (a data: URL, which
// html/template would render as "#ZgotmplZ", or junk) is "", so the page
// falls back to grain's own image instead of a broken one.
func webImageURL(v string) string {
	v = strings.TrimSpace(v)
	if strings.ContainsAny(v, "\\\r\n\t") {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	switch {
	case (u.Scheme == "http" || u.Scheme == "https") && u.Host != "":
		return v
	case u.Scheme == "" && u.Host == "" && strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//"):
		return v
	}
	return ""
}

func brandingName() string      { return currentBranding().Name }
func brandingIcon() string      { return currentBranding().Icon }
func brandingLogo() string      { return currentBranding().Logo }
func brandingLogoLight() string { return currentBranding().LogoLight }

// RelayPageTitle is the browser-tab title for the relay's own pages: the
// relay name when one is configured, otherwise grain's default.
func RelayPageTitle() string {
	if n := brandingName(); n != "" {
		return n
	}
	return defaultBrandName
}

// assetVersion is stamped onto static asset URLs (?v=<version>) in the
// layout so a new release gets a fresh URL the CDN / browser / service
// worker can't stale-serve. Set once at startup from the build version.
var assetVersion = "dev"

// SetAssetVersion wires the build version in so {{assetVersion}} in the
// templates resolves to it. Called from main once the version is known.
//
// Dev builds carry no release version (Version == "dev"), and a fixed "dev"
// stamp lets the browser and the PWA service worker stale-serve old JS across
// rebuilds — the SW caches some scripts cache-first, so a constant ?v=dev URL
// never re-fetches. Use a per-process value for dev instead, so every rebuild /
// restart produces fresh ?v= URLs that bust both caches.
func SetAssetVersion(v string) {
	if v == "" || v == "dev" {
		assetVersion = "dev-" + strconv.FormatInt(time.Now().Unix(), 10)
		return
	}
	assetVersion = v
}

// baseTemplateFuncs are the helpers every layout-rendered page needs.
// All three parse sites (RenderTemplate, renderAdmin, renderSetup) must
// register these or ParseFS fails on the {{assetVersion}} reference in
// layout.html.
var baseTemplateFuncs = template.FuncMap{
	"assetVersion": func() string { return assetVersion },
	// relayName / relayIcon / relayLogo expose the relay's configured
	// branding (relay_metadata.json) to the shared layout (favicon, PWA
	// meta) and header (logo / name). Empty when unset, so each falls back
	// to grain's own. See layout.html and header.html.
	"relayName": brandingName,
	"relayIcon": brandingIcon,
	"relayLogo": brandingLogo,
	// relayLogoLight: the header logo under light themes (optional).
	"relayLogoLight": brandingLogoLight,
}

// Define the base directories for views and templates within the embedded FS
const (
	viewsDir     = "www/views"
	templatesDir = "www/views/templates"
)

// Define the common layout template filenames
var templateFiles = []string{
	"layout.html",
	"header.html",
	"footer.html",
}

// layoutPatterns returns the full embedded FS paths for layout templates
func layoutPatterns() []string {
	var paths []string
	for _, file := range templateFiles {
		paths = append(paths, path.Join(templatesDir, file))
	}
	return paths
}

// RenderTemplate renders a template with the standard layout using the embedded FS
func RenderTemplate(w http.ResponseWriter, data PageData, view string) {
	viewTemplate := path.Join(viewsDir, view)
	componentPattern := path.Join(viewsDir, "components", "*.html")

	componentTemplates, err := fs.Glob(wwwFS, componentPattern)
	if err != nil {
		http.Error(w, "Error loading component templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	patterns := append(layoutPatterns(), viewTemplate)
	patterns = append(patterns, componentTemplates...)

	tmpl, err := template.New("").Funcs(baseTemplateFuncs).ParseFS(wwwFS, patterns...)
	if err != nil {
		http.Error(w, "Error parsing templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
}
