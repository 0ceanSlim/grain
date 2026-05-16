// Package docs serves grain's OpenAPI specification.
//
// The spec is generated at build time by `swag init` (see the root
// Makefile's `generate` target) from doc comments scattered across
// the HTTP handlers, then embedded into the binary via //go:embed in
// main.go. This package is the runtime side of that pipeline: it
// holds the spec bytes and serves them.
//
// The HTML shell that consumes the spec — grain's restyled Swagger
// UI — lives in www/views/api-docs.html and is rendered through the
// standard template engine from client/registerEndpoints.go. The
// swagger-ui-dist JS/CSS bundle is downloaded into www/static/swagger
// at build time (see tests/docker/Dockerfile and the assets job in
// .github/workflows/release.yml). Keeping the runtime here narrow
// means swap-out is just an HTML rewrite, no Go changes.
package docs

import (
	"net/http"

	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
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
