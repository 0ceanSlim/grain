// Package docs serves grain's OpenAPI specification and the Swagger
// UI that consumes it.
//
// The spec is generated at build time by `swag init` (see the root
// Makefile's `generate` target) from doc comments scattered across
// the HTTP handlers, then embedded into the binary via //go:embed in
// main.go. This package is the runtime side of that pipeline: it
// holds the spec bytes and serves them, plus the Swagger UI HTML
// shell that points back at the JSON URL.
//
// The reason it lives in its own subpackage (rather than alongside
// the rest of server/api) is that github.com/swaggo/http-swagger
// pulls in transitively github.com/swaggo/files, which only matters
// when docs are served. Keeping that import out of the main api
// package avoids surprising downstream callers with a heavyweight
// dependency they don't need.
package docs

import (
	"net/http"

	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// spec holds the raw OpenAPI JSON bytes. Wired in main.go via SetSpec
// during startup. A nil spec is treated as a runtime misconfiguration
// (build forgot to run `make generate`) and returns 503 rather than
// silently serving an empty body — the operator should see the
// problem loudly.
var spec []byte

// SetSpec installs the embedded OpenAPI JSON. Called once during
// startup; not safe for concurrent use, which matches the existing
// SetEmbeddedWWW pattern in client/embed.go.
func SetSpec(b []byte) { spec = b }

// ServeSpec writes the embedded OpenAPI JSON. Mounted at
// /api/docs/openapi.json so the Swagger UI can fetch it
// same-origin (no CORS dance for the dashboard).
func ServeSpec(w http.ResponseWriter, r *http.Request) {
	if len(spec) == 0 {
		log.RelayAPI().Error("OpenAPI spec is empty",
			"client_ip", utils.GetClientIP(r),
			"hint", "ensure `make generate` ran before `go build`")
		http.Error(w, "OpenAPI spec not available", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	if _, err := w.Write(spec); err != nil {
		log.RelayAPI().Error("Failed to write OpenAPI spec",
			"client_ip", utils.GetClientIP(r),
			"error", err)
	}
}

// UIHandler returns an http.Handler that serves the Swagger UI shell
// pointing at our embedded spec. swaggo's handler embeds its own
// copy of the UI assets, so no separate static-file setup is needed.
//
// The handler is registered at /api/docs/ — the trailing slash
// matters: http.ServeMux's tree treats it as a subtree match so
// requests to /api/docs/index.html, /api/docs/swagger-ui.css, etc.
// all resolve to the swaggo handler without our having to enumerate
// each asset.
func UIHandler() http.Handler {
	return httpSwagger.Handler(
		httpSwagger.URL("/api/docs/openapi.json"),
		// DeepLinking lets ?tag-name= URLs scroll directly to a
		// section, which is handy when linking to a method from
		// chat or a PR description.
		httpSwagger.DeepLinking(true),
	)
}
