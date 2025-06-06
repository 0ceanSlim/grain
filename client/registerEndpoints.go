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
	mux.HandleFunc("/api/v1/cache", api.GetCacheHandler)  	 // Get cached user data

	mux.HandleFunc("/api/v1/whitelist/pubkeys", api.GetAllWhitelistedPubkeys)
	mux.HandleFunc("/api/v1/blacklist/pubkeys", api.GetAllBlacklistedPubkeys)

	// auth endpoints
	mux.HandleFunc("/login", auth.LoginHandler)
	mux.HandleFunc("/logout", auth.LogoutHandler)

	// route endpoints are registerd with a function inside it's package 
	// to avoid circular imports

	// function endpoints
	// will implement later. these will be core nostr client functions
}
