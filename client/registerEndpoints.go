// client/registerEndpoints.go
package client

import (
	"net/http"

	"github.com/0ceanslim/grain/client/api"
	relay "github.com/0ceanslim/grain/server/api"
	docs "github.com/0ceanslim/grain/server/api/docs"
)

// RegisterEndpoints registers all endpoints on the given mux
func RegisterEndpoints(mux *http.ServeMux) {

	// client api endpoints
	mux.HandleFunc("/api/v1/session", api.GetSessionHandler) // Get current session info
	// Cache endpoints
	mux.HandleFunc("/api/v1/cache", api.GetCacheHandler)             // GET for cache data
	mux.HandleFunc("/api/v1/cache/refresh", api.RefreshCacheHandler) // POST for manual refresh

	// Auth API endpoints
	mux.HandleFunc("/api/v1/auth/login", api.LoginHandler)   // Login via API
	mux.HandleFunc("/api/v1/auth/logout", api.LogoutHandler) // Logout via API
	// Amber NIP-55 callback endpoint
	mux.HandleFunc("/api/v1/auth/amber-callback", api.HandleAmberCallback) // Amber signer callback (NIP-55)

	// relay api endpoints - key management (cached)
	mux.HandleFunc("/api/v1/relay/keys/whitelist", relay.GetAllWhitelistedPubkeys)
	mux.HandleFunc("/api/v1/relay/keys/blacklist", relay.GetAllBlacklistedPubkeys)

	// relay api endpoints - key management (live)
	mux.HandleFunc("/api/v1/relay/keys/whitelist/live", relay.GetAllWhitelistedPubkeysLive)
	mux.HandleFunc("/api/v1/relay/keys/blacklist/live", relay.GetAllBlacklistedPubkeysLive)

	// relay configuration endpoints (read-only)
	mux.HandleFunc("/api/v1/relay/config/server", relay.GetServerConfig)
	mux.HandleFunc("/api/v1/relay/config/rate_limit", relay.GetRateLimitConfig)
	mux.HandleFunc("/api/v1/relay/config/event_purge", relay.GetEventPurgeConfig)
	mux.HandleFunc("/api/v1/relay/config/logging", relay.GetLoggingConfig)
	mux.HandleFunc("/api/v1/relay/config/resource_limits", relay.GetResourceLimitsConfig)
	mux.HandleFunc("/api/v1/relay/config/auth", relay.GetAuthConfig)
	mux.HandleFunc("/api/v1/relay/config/event_time_constraints", relay.GetEventTimeConstraintsConfig)
	mux.HandleFunc("/api/v1/relay/config/backup_relay", relay.GetBackupRelayConfig)
	mux.HandleFunc("/api/v1/relay/config/whitelist", relay.GetWhitelistConfig)
	mux.HandleFunc("/api/v1/relay/config/blacklist", relay.GetBlacklistConfig)

	// Key generation endpoint
	mux.HandleFunc("/api/v1/keys/generate", api.KeyGenerationHandler) // Generate random key pair

	// Key derivation endpoint
	mux.HandleFunc("/api/v1/keys/derive/", api.KeyDeriveHandler) // Derive public key from private key

	// Key conversion endpoints
	mux.HandleFunc("/api/v1/keys/convert/public/", api.PublicKeyConversionHandler)   // Convert hex ↔ npub
	mux.HandleFunc("/api/v1/keys/convert/private/", api.PrivateKeyConversionHandler) // Convert hex ↔ nsec

	// Key validation endpoint
	mux.HandleFunc("/api/v1/keys/validate/", api.KeyValidationHandler) // Validate any key type

	mux.HandleFunc("/api/v1/ping/", api.PingHandler)
	mux.HandleFunc("/api/v1/keys/decode/nip19/", api.Nip19DecodeHandler) // Decode NIP-19 entities

	// OpenAPI / Swagger UI. The spec is served standalone so other
	// tooling (Postman, openapi-generator, etc.) can consume it
	// without scraping the UI shell. The UI itself is grain's own
	// HTML shell that loads swagger-ui-dist from /static/swagger/ and
	// applies the design tokens — the swaggo runtime is no longer
	// used; only its CLI participates, at build time, to generate the
	// spec embedded into the binary.
	mux.HandleFunc("/api/docs/openapi.json", docs.ServeSpec)
	mux.HandleFunc("/api/docs/", apiDocsHandler)
	mux.HandleFunc("/api/docs", apiDocsHandler)

	// Style tester. Renders every design token + common widget
	// against the active theme. No auth gate — kept in the binary as
	// a permanent dev/preview tool.
	mux.HandleFunc("/style-test", styleTestHandler)

	// Core Nostr client function endpoints
	registerCoreClientEndpoints(mux)

}

// apiDocsHandler serves grain's Swagger UI shell. The actual
// SwaggerUIBundle JS and base CSS are loaded by the template from
// /static/swagger/; grain's overrides go on top via
// /static/css/swagger-overrides.css. The handler ignores any path
// beyond /api/docs/ — assets are mounted under /static/swagger/
// instead, so a request to /api/docs/swagger-ui.css won't accidentally
// hit this template renderer.
func apiDocsHandler(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, PageData{Title: "🌾 grain — API docs"}, "api-docs.html")
}

// styleTestHandler renders the design-token preview page. Bypasses the
// HTMX SPA shell so the layout (header + theme swapper + footer)
// wraps the tester even on direct navigation.
func styleTestHandler(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, PageData{Title: "🌾 grain — style tester"}, "style-test.html")
}

// registerCoreClientEndpoints registers endpoints for core Nostr client functions
func registerCoreClientEndpoints(mux *http.ServeMux) {
	// Event publishing endpoints
	mux.HandleFunc("/api/v1/publish", api.PublishEventHandler)

	// User data fetching endpoints
	mux.HandleFunc("/api/v1/user/profile", api.GetUserProfileHandler)
	mux.HandleFunc("/api/v1/user/relays", api.GetUserRelaysHandler)

	// Event querying endpoints
	mux.HandleFunc("/api/v1/events/query", api.QueryEventsHandler)

	// Relay management endpoints
	mux.HandleFunc("/api/v1/client/relays", api.ClientRelaysHandler)
	mux.HandleFunc("/api/v1/client/connect/", api.ClientConnectHandler)
	mux.HandleFunc("/api/v1/client/disconnect/", api.ClientDisconnectHandler)
}
