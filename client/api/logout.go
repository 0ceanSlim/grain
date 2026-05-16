package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// LogoutHandler handles user logout requests via API
//
// @Summary      Log out
// @Description  Clears the active session cookie. Idempotent — succeeds even if no session was active.
// @Tags         client-auth
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      405  {string}  string  "Method not allowed"
// @Router       /api/v1/auth/logout [post]
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.ClientAPI().Debug("API logout handler called")

	// Get current user session
	user := session.SessionMgr.GetCurrentUser(r)
	if user != nil {
		log.ClientAPI().Info("User logging out",
			"pubkey", user.PublicKey,
			"mode", user.Mode,
			"signing_method", user.SigningMethod)
	}

	// Clear the session
	session.SessionMgr.ClearSession(w, r)

	response := map[string]interface{}{
		"success": true,
		"message": "Logout successful",
	}

	log.ClientAPI().Info("API logout successful")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
