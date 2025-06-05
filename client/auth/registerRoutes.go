package auth

import "net/http"

// RegisterAuthEndpoints registers all API endpoints on the given mux
func RegisterAuthEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("/do-login", LoginHandler)
	mux.HandleFunc("/logout", LogoutHandler)

}
