package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetSessionHandler returns the current user's session data as JSON (auth state only)
func GetSessionHandler(w http.ResponseWriter, r *http.Request) {
	// Get current session
	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil {
		http.Error(w, "No active session found", http.StatusUnauthorized)
		log.ClientAPI().Debug("No active session found for request")
		return
	}

	// Create minimal session response (auth state only)
	sessionData := map[string]interface{}{
		"publicKey":     userSession.PublicKey,
		"lastActive":    userSession.LastActive,
		"mode":          userSession.Mode,
		"signingMethod": userSession.SigningMethod,
	}

	log.ClientAPI().Debug("Returning session data",
		"pubkey", userSession.PublicKey,
		"mode", userSession.Mode)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sessionData); err != nil {
		log.ClientAPI().Error("Failed to encode session data", "error", err)
		http.Error(w, "Failed to retrieve session data", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Session data retrieved successfully", "pubkey", userSession.PublicKey)
}
