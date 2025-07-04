package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetSessionHandler returns the current user's  session data as JSON
func GetSessionHandler(w http.ResponseWriter, r *http.Request) {
	// Get current session
	session := session.SessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "No active session found", http.StatusUnauthorized)
		log.ClientAPI().Debug("No active session found for request")
		return
	}

	// Get fresh relay info from cache
	var relayInfo map[string]interface{}
	if cachedData, found := cache.GetUserData(session.PublicKey); found {
		var mailboxes core.Mailboxes
		if err := json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxes); err == nil {
			userRelays := mailboxes.ToStringSlice()
			relayInfo = map[string]interface{}{
				"userRelays": userRelays,
				"relayCount": len(userRelays),
				"read":       mailboxes.Read,
				"both":       mailboxes.Both,
			}
			// Note: removed the redundant "write": mailboxes.Write that was showing as null
		}
	}

	// Create comprehensive session response
	sessionData := map[string]interface{}{
		"publicKey":       session.PublicKey,
		"lastActive":      session.LastActive,
		"mode":            session.Mode,
		"signingMethod":   session.SigningMethod,
		"connectedRelays": session.ConnectedRelays,
	}

	// Add relay info if available
	if relayInfo != nil {
		sessionData["relays"] = relayInfo
	}

	log.ClientAPI().Debug("Returning  session data",
		"pubkey", session.PublicKey,
		"mode", session.Mode)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sessionData); err != nil {
		log.ClientAPI().Error("Failed to encode session data", "error", err)
		http.Error(w, "Failed to retrieve session data", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info(" session data retrieved successfully", "pubkey", session.PublicKey)
}
