package client

import (
	"encoding/json"
	"net/http"
	"path/filepath"

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
	Type    string `json:"type"`
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
		BackgroundColor: "#1f2937",
		ThemeColor:      "#8b5cf6",
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

	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		log.ClientMain().Error("Failed to encode PWA manifest", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientMain().Debug("Served PWA manifest", "client_ip", r.RemoteAddr)
}

// serviceWorkerHandler serves the service worker JavaScript file
func serviceWorkerHandler(w http.ResponseWriter, r *http.Request) {
	swPath := filepath.Join("www", "sw.js")

	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // SW should not be cached
	w.Header().Set("Service-Worker-Allowed", "/")                          // Allow SW to control entire origin

	http.ServeFile(w, r, swPath)

	log.ClientMain().Debug("Served service worker", "client_ip", r.RemoteAddr)
}

// RegisterPWARoutes registers PWA-related routes
func RegisterPWARoutes(mux *http.ServeMux) {
	mux.HandleFunc("/manifest.json", manifestHandler)
	mux.HandleFunc("/sw.js", serviceWorkerHandler)

	log.ClientMain().Info("PWA routes registered", "routes", []string{"/manifest.json", "/sw.js"})
}
