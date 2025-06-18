// client/registerEndpoints.go
package client

import (
	"net/http"

	"github.com/0ceanslim/grain/client/api"
	"github.com/0ceanslim/grain/client/auth"
)

// RegisterEndpoints registers all endpoints on the given mux
func RegisterEndpoints(mux *http.ServeMux) {

	// api endpoints
	mux.HandleFunc("/api/v1/session", api.GetSessionHandler)         // Get current session info
	mux.HandleFunc("/api/v1/cache", api.GetCacheHandler)  	        // Get cached user data

	mux.HandleFunc("/api/v1/whitelist/pubkeys", api.GetAllWhitelistedPubkeys)
	mux.HandleFunc("/api/v1/blacklist/pubkeys", api.GetAllBlacklistedPubkeys)

	// auth endpoints
	mux.HandleFunc("/login", auth.LoginHandler)
	mux.HandleFunc("/logout", auth.LogoutHandler)

	// route endpoints are registered with a function inside it's package 
	// to avoid circular imports

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