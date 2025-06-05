package routes

import "net/http"

// RegisterViewRoutes registers all views on the given mux
func RegisterViewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/profile", ProfileHandler)
}
