package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetSessionHandler returns the current user's session data as JSON
func GetSessionHandler(w http.ResponseWriter, r *http.Request) {
	// Get current session
	session := auth.SessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "No active session found", http.StatusUnauthorized)
		log.Util().Debug("No active session found for request")
		return
	}

	// Get relay info from cache instead of session
	var relayInfo map[string]interface{}
	if cachedData, found := cache.GetUserData(session.PublicKey); found {
		var mailboxes core.Mailboxes
		if err := json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxes); err == nil {
			userRelays := mailboxes.ToStringSlice()
			relayInfo = map[string]interface{}{
				"userRelays": userRelays,
				"relayCount": len(userRelays),
				"read":       mailboxes.Read,
				"write":      mailboxes.Write,
				"both":       mailboxes.Both,
			}
		}
	}

	// Create lightweight session response
	sessionData := map[string]interface{}{
		"publicKey":  session.PublicKey,
		"lastActive": session.LastActive,
	}

	// Add relay info if available
	if relayInfo != nil {
		sessionData["relays"] = relayInfo
	}

	log.Util().Debug("Returning session data", "pubkey", session.PublicKey)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sessionData); err != nil {
		log.Util().Error("Failed to encode session data", "error", err)
		http.Error(w, "Failed to retrieve session data", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Session data retrieved successfully", "pubkey", session.PublicKey)
}