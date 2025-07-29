package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/data"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetCacheHandler returns the cached user data as JSON (no session data)
// Automatically refreshes cache if expired or missing
func GetCacheHandler(w http.ResponseWriter, r *http.Request) {
	// Get current session to identify user
	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}

	publicKey := userSession.PublicKey
	log.ClientAPI().Debug("Getting cached user data", "pubkey", publicKey)

	// Get cached data
	cachedData, found := cache.GetUserData(publicKey)
	if !found {
		log.ClientAPI().Info("Cache miss, rebuilding from session key", "pubkey", publicKey)

		// Attempt to rebuild cache
		if err := data.FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
			log.ClientAPI().Error("Failed to rebuild cache", "pubkey", publicKey, "error", err)
			http.Error(w, "Failed to load user data", http.StatusInternalServerError)
			return
		}

		// Try to get cache again after rebuild
		cachedData, found = cache.GetUserData(publicKey)
		if !found {
			log.ClientAPI().Error("Cache still empty after rebuild", "pubkey", publicKey)
			http.Error(w, "Failed to load user data", http.StatusInternalServerError)
			return
		}

		log.ClientAPI().Info("Cache rebuilt successfully", "pubkey", publicKey)
	}

	// Parse metadata
	var parsedMetadata map[string]interface{}
	if cachedData.Metadata != "" {
		if err := json.Unmarshal([]byte(cachedData.Metadata), &parsedMetadata); err != nil {
			log.ClientAPI().Warn("Failed to parse cached metadata", "pubkey", publicKey, "error", err)
		}
	}

	// Parse mailboxes (raw Nostr data)
	var parsedMailboxes map[string]interface{}
	if cachedData.Mailboxes != "" && cachedData.Mailboxes != "{}" {
		if err := json.Unmarshal([]byte(cachedData.Mailboxes), &parsedMailboxes); err != nil {
			log.ClientAPI().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
		}
	}

	// Get user's client relays (the managed relay list that the app actually uses)
	var clientRelays map[string]interface{}
	if userClientRelays, err := cache.GetUserClientRelays(publicKey); err == nil && len(userClientRelays) > 0 {
		connected := 0
		relayList := make([]map[string]interface{}, len(userClientRelays))

		for i, relay := range userClientRelays {
			if relay.Connected {
				connected++
			}
			relayList[i] = map[string]interface{}{
				"url":       relay.URL,
				"read":      relay.Read,
				"write":     relay.Write,
				"connected": relay.Connected,
				"added_at":  relay.AddedAt,
			}
		}

		clientRelays = map[string]interface{}{
			"relays":    relayList,
			"total":     len(userClientRelays),
			"connected": connected,
		}
	}

	// Encode npub for display
	npub, err := tools.EncodePubkey(publicKey)
	if err != nil {
		log.ClientAPI().Error("Failed to encode npub", "pubkey", publicKey, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create comprehensive cache response
	response := map[string]interface{}{
		"publicKey":      publicKey,
		"npub":           npub,
		"cacheTimestamp": cachedData.Timestamp,
		"cacheAge":       cachedData.Timestamp.Format("2006-01-02 15:04:05"),
		"refreshed":      !found,
		// Include session information
		"sessionMode":   userSession.Mode,
		"signingMethod": userSession.SigningMethod,
	}

	// Add metadata
	if parsedMetadata != nil {
		response["metadata"] = parsedMetadata
	} else if cachedData.Metadata != "" {
		response["metadataRaw"] = cachedData.Metadata
	}

	// Add mailboxes (raw Nostr relay list data from kind 10002)
	if parsedMailboxes != nil {
		response["mailboxes"] = parsedMailboxes
	} else if cachedData.Mailboxes != "" {
		response["mailboxesRaw"] = cachedData.Mailboxes
	}

	// Add clientRelays (the app's managed relay list derived from mailboxes)
	// This is the ONLY relay field we need - it includes all the relay info the client needs
	if clientRelays != nil {
		response["clientRelays"] = clientRelays
	}

	log.ClientAPI().Debug("Returning cached user data",
		"pubkey", publicKey,
		"cache_age", cachedData.Timestamp.Format("15:04:05"))

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode cached data", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to retrieve cached data", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Cached user data retrieved successfully", "pubkey", publicKey)
}

// RefreshCacheHandler manually refreshes cache for the current user
func RefreshCacheHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current session to identify user
	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}

	publicKey := userSession.PublicKey
	log.ClientAPI().Info("Manual cache refresh requested", "pubkey", publicKey)

	// Clear existing cache first
	cache.ClearUserData(publicKey)

	// Rebuild cache from session key
	if err := data.FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
		log.ClientAPI().Error("Manual cache refresh failed", "pubkey", publicKey, "error", err)

		response := map[string]interface{}{
			"message": "Failed to refresh cache: " + err.Error(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"message": "Cache refreshed successfully",
		"pubkey":  publicKey,
	}

	log.ClientAPI().Info("Manual cache refresh successful", "pubkey", publicKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
