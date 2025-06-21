// client/auth/logout.go
package auth

import (
	"net/http"

	"github.com/0ceanslim/grain/server/utils/log"
)

// LogoutHandler handles user logout and session cleanup (DEPRECATED - use LegacyLogoutHandler)
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Logout handler called (deprecated)")

	// Get current user session using the EnhancedSessionManager
	user := EnhancedSessionMgr.GetCurrentUser(r)
	if user != nil {
		log.Util().Info("User logging out", "pubkey", user.PublicKey)
	}

	// Clear the session using the EnhancedSessionManager
	EnhancedSessionMgr.ClearSession(w, r)

	log.Util().Info("User session cleared successfully")

	// Return success response for HTMX instead of redirect
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful"))
	log.Util().Debug("Logout successful response sent")
}