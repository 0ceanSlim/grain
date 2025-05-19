package api

import "net/http"

// RegisterAPIRoutes registers all API endpoints on the given mux
func RegisterAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/whitelist/pubkeys", GetAllWhitelistedPubkeys)
	mux.HandleFunc("/api/v1/blacklist/pubkeys", GetAllBlacklistedPubkeys)
}
