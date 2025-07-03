// client/registerEndpoints.go
package client

import (
	"net/http"

	"github.com/0ceanslim/grain/client/api"
	relay "github.com/0ceanslim/grain/server/api"
)

// RegisterEndpoints registers all endpoints on the given mux
func RegisterEndpoints(mux *http.ServeMux) {

	// client api endpoints
	mux.HandleFunc("/api/v1/session", api.GetSessionHandler)         // Get current session info
	// Cache endpoints
	mux.HandleFunc("/api/v1/cache", api.GetCacheHandler)           // GET for cache data
	mux.HandleFunc("/api/v1/cache/refresh", api.RefreshCacheHandler) // POST for manual refresh

	// Auth API endpoints (preferred)
	mux.HandleFunc("/api/v1/auth/login", api.LoginHandler)      // Login via API
	mux.HandleFunc("/api/v1/auth/logout", api.LogoutHandler)    // Logout via API
	// Amber NIP-55 callback endpoint
	mux.HandleFunc("/api/v1/auth/amber-callback", api.HandleAmberCallback) // Amber signer callback (NIP-55)

	// relay api endpoints
	mux.HandleFunc("/api/v1/relay/whitelist", relay.GetAllWhitelistedPubkeys)
	mux.HandleFunc("/api/v1/relay/blacklist", relay.GetAllBlacklistedPubkeys)

	// Key generation endpoint
	mux.HandleFunc("/api/v1/generate/keypair", api.GenerateKeypairHandler) // Generate random key pair

	// Key conversion endpoints
	mux.HandleFunc("/api/v1/convert/pubkey", api.ConvertPubkeyHandler)   // Convert pubkey to npub
	mux.HandleFunc("/api/v1/convert/npub", api.ConvertNpubHandler)       // Convert npub to pubkey
	
	// Key validation endpoints
	mux.HandleFunc("/api/v1/validate/pubkey", api.ValidatePubkeyHandler) // Validate pubkey
	mux.HandleFunc("/api/v1/validate/npub", api.ValidateNpubHandler)     // Validate npub

	// Core Nostr client function endpoints
	registerCoreClientEndpoints(mux)
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
	mux.HandleFunc("/api/v1/relays/connect", api.ConnectRelayHandler)
	mux.HandleFunc("/api/v1/relays/disconnect", api.DisconnectRelayHandler)
	mux.HandleFunc("/api/v1/relays/status", api.GetRelayStatusHandler)
}