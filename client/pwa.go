package client

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/0ceanslim/grain/server/utils/log"
)

// PWAManifest represents the web app manifest structure
type PWAManifest struct {
	Name            string          `json:"name"`
	ShortName       string          `json:"short_name"`
	Description     string          `json:"description"`
	StartURL        string          `json:"start_url"`
	Display         string          `json:"display"`
	BackgroundColor string          `json:"background_color"`
	ThemeColor      string          `json:"theme_color"`
	Orientation     string          `json:"orientation"`
	Scope           string          `json:"scope"`
	Lang            string          `json:"lang"`
	Categories      []string        `json:"categories"`
	Screenshots     []PWAScreenshot `json:"screenshots,omitempty"`
	Icons           []PWAIcon       `json:"icons"`
}

// PWAIcon represents an icon in the manifest
type PWAIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

// PWAScreenshot represents a screenshot in the manifest
type PWAScreenshot struct {
	Src        string `json:"src"`
	Sizes      string `json:"sizes"`
	Type       string `json:"type"`
	FormFactor string `json:"form_factor,omitempty"`
	Label      string `json:"label,omitempty"`
}

// manifestHandler serves the PWA manifest.json file
func manifestHandler(w http.ResponseWriter, r *http.Request) {
	manifest := PWAManifest{
		Name:            "Grain - Nostr Relay",
		ShortName:       "Grain",
		Description:     "Go Relay Architecture for Implementing Nostr",
		StartURL:        "/",
		Display:         "standalone",
		BackgroundColor: "#0d0f0c",
		ThemeColor:      "#141710",
		Orientation:     "portrait-primary",
		Scope:           "/",
		Lang:            "en",
		Categories:      []string{"social", "utilities"},
		Icons: []PWAIcon{
			{Src: "/static/icons/icon-72x72.png", Sizes: "72x72", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-96x96.png", Sizes: "96x96", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-128x128.png", Sizes: "128x128", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-144x144.png", Sizes: "144x144", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-152x152.png", Sizes: "152x152", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-192x192.png", Sizes: "192x192", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-384x384.png", Sizes: "384x384", Type: "image/png", Purpose: "maskable any"},
			{Src: "/static/icons/icon-512x512.png", Sizes: "512x512", Type: "image/png", Purpose: "maskable any"},
		},
	}

	applyRelayBranding(&manifest, currentBranding())

	w.Header().Set("Content-Type", "application/manifest+json")
	// One hour: the manifest follows the relay's icon and name, so a change
	// made in /admin reaches installs within the hour.
	w.Header().Set("Cache-Control", "public, max-age=3600")

	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		log.ClientMain().Error("Failed to encode PWA manifest", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientMain().Debug("Served PWA manifest", "client_ip", r.RemoteAddr)
}

// applyRelayBranding makes an installed app look like the relay when the
// operator has set a relay icon (NIP-11 icon, the admin "Favicon"): the relay
// name becomes the app name, and a PNG or WebP icon is offered first at the
// two sizes install prompts look for (the admin form asks for square, 512px
// or larger); purpose "any", since an arbitrary image has no maskable safe
// zone. grain's own icons stay listed after it, so the app stays installable
// whatever the relay's image turns out to be.
func applyRelayBranding(m *PWAManifest, b RelayBranding) {
	if b.Icon == "" {
		return
	}
	if typ := iconMIMEType(b.Icon); typ == "image/png" || typ == "image/webp" {
		m.Icons = append([]PWAIcon{
			{Src: b.Icon, Sizes: "192x192", Type: typ, Purpose: "any"},
			{Src: b.Icon, Sizes: "512x512", Type: typ, Purpose: "any"},
		}, m.Icons...)
	}
	if b.Name != "" {
		m.Name = b.Name
		m.ShortName = b.Name
	}
}

// iconMIMEType guesses an icon's type from its URL path extension; empty
// (type omitted, the browser sniffs) when it cannot tell.
func iconMIMEType(src string) string {
	p := src
	if u, err := url.Parse(src); err == nil {
		p = u.Path
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".ico":
		return "image/x-icon"
	}
	return ""
}

// serviceWorkerHandler serves the service worker JavaScript file from the embedded FS
func serviceWorkerHandler(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(wwwFS, "www/sw.js")
	if err != nil {
		log.ClientMain().Error("Failed to read embedded service worker", "error", err)
		http.Error(w, "Service worker not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // SW should not be cached
	w.Header().Set("Service-Worker-Allowed", "/")                          // Allow SW to control entire origin

	w.Write(data)

	log.ClientMain().Debug("Served service worker", "client_ip", r.RemoteAddr)
}

// RegisterPWARoutes registers PWA-related routes
func RegisterPWARoutes(mux *http.ServeMux) {
	mux.HandleFunc("/manifest.json", manifestHandler)
	mux.HandleFunc("/sw.js", serviceWorkerHandler)

	log.ClientMain().Info("PWA routes registered", "routes", []string{"/manifest.json", "/sw.js"})
}
