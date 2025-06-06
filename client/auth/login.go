package auth

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Global session manager instance - initialize this in your startup code
var SessionMgr *SessionManager

// Application relays for fetching user data - initialize this in your startup code
var appRelays []string

// LoginHandler handles user login and initialization
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Login handler called")

	// Check if user is already logged in
	token := SessionMgr.GetSessionToken(r)
	if token != "" {
		session := SessionMgr.GetUserSession(token)
		if session != nil {
			log.Util().Info("User already logged in", "pubkey", session.PublicKey)
			return
		}
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Util().Error("Failed to parse form", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	publicKey := r.FormValue("publicKey")
	if publicKey == "" {
		log.Util().Warn("Missing publicKey in form data")
		http.Error(w, "Missing publicKey", http.StatusBadRequest)
		return
	}
	log.Util().Info("Received publicKey", "pubkey", publicKey)

	// Try to get user data from cache first - ACTUALLY USE THE CACHED DATA
	cachedUserData, exists := cache.GetUserData(publicKey)
	if exists {
		// Parse cached data (following original validation logic exactly)
		var userMetadata nostr.Event // Adjust type to match your types
		if err := json.Unmarshal([]byte(cachedUserData.Metadata), &userMetadata); err != nil {
			log.Util().Warn("Failed to parse cached metadata", "pubkey", publicKey, "error", err)
		} else {
			var mailboxes core.Mailboxes // Adjust type to match your types
			if err := json.Unmarshal([]byte(cachedUserData.Mailboxes), &mailboxes); err != nil {
				log.Util().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
			} else {
				// Create session with cached data - CRITICAL: only return if successful
				session, err := SessionMgr.CreateSession(w, publicKey)
				if err == nil {
					log.Util().Info("Using cached data", "pubkey", session.PublicKey)
					return // ✅ SUCCESS: Using validated cached data
				}
				// ✅ If session creation fails, continue to fetch fresh data (original behavior)
				log.Util().Warn("Failed to create session with cached data, fetching fresh", "pubkey", publicKey, "error", err)
			}
		}
	}

	// Fall back to fetching data from relays
	log.Util().Debug("Fetching user data from relays", "pubkey", publicKey)

	// Fetch user mailboxes
	mailboxes, err := core.FetchUserMailboxes(publicKey, appRelays)
	if err != nil {
		log.Util().Error("Failed to fetch user relays", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to fetch user relays", http.StatusInternalServerError)
		return
	}

	// Build relay list exactly like original
	allRelays := append(mailboxes.Read, mailboxes.Write...)
	allRelays = append(allRelays, mailboxes.Both...)
	log.Util().Info("Fetched user relays", "pubkey", publicKey, "mailboxes", mailboxes)

	userMetadata, err := core.FetchUserMetadata(publicKey, allRelays)
	if err != nil || userMetadata == nil {
		log.Util().Error("Failed to fetch user metadata", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to fetch user metadata", http.StatusInternalServerError)
		return
	}

	// Cache the user data (following original parameter order)
	kind10002JSON, _ := json.Marshal(mailboxes)
	kind0JSON, _ := json.Marshal(userMetadata)
	cache.SetUserData(publicKey, string(kind0JSON), string(kind10002JSON))
	log.Util().Debug("User data cached successfully", "pubkey", publicKey)

	// Create new session
	session, err := SessionMgr.CreateSession(w, publicKey)
	if err != nil {
		log.Util().Error("Failed to create session", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	log.Util().Info("User logged in successfully", "pubkey", session.PublicKey)
}