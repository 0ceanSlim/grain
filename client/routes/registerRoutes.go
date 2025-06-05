package routes

import "net/http"

// RegisterAPIRoutes registers all API endpoints on the given mux
func RegisterViewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/profile", ProfileHandler)
}
