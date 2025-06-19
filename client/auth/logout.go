package auth

import (
	"net/http"

	"github.com/0ceanslim/grain/server/utils/log"
)

// LogoutHandler handles user logout and session cleanup
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Logout handler called")

	// Get current user session using the SessionManager method
	user := SessionMgr.GetCurrentUser(r)
	if user != nil {
		log.Util().Info("User logging out", "pubkey", user.PublicKey)
	}

	// Clear the session using the SessionManager
	SessionMgr.ClearSession(w, r)

	log.Util().Info("User session cleared successfully")

	// Return success response for HTMX instead of redirect
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful"))
	log.Util().Debug("Logout successful response sent")
}