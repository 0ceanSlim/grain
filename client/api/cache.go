package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
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
		log.ClientAPI().Debug("No active session found for cache request")
		return
	}

	publicKey := userSession.PublicKey
	cachedData, found := cache.GetUserData(publicKey)

	// If cache is missing, rebuild it from the user's session key
	if !found {
		log.ClientAPI().Info("Cache missing, rebuilding from user session", "pubkey", publicKey)

		// Fetch fresh data from Nostr network using session key
		if err := data.FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
			log.ClientAPI().Error("Failed to rebuild cache", "pubkey", publicKey, "error", err)

			// Return error response
			response := map[string]interface{}{
				"error":   "Failed to load user data",
				"message": "Could not fetch data from Nostr network",
				"pubkey":  publicKey,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Get cached data after rebuild
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

	// Parse mailboxes and create relay info
	var parsedMailboxes map[string]interface{}
	var relays map[string]interface{}
	if cachedData.Mailboxes != "" && cachedData.Mailboxes != "{}" {
		if err := json.Unmarshal([]byte(cachedData.Mailboxes), &parsedMailboxes); err != nil {
			log.ClientAPI().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
		} else {
			// Create structured relay info from mailboxes
			var mailboxes core.Mailboxes
			if json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxes) == nil {
				userRelays := mailboxes.ToStringSlice()
				relays = map[string]interface{}{
					"userRelays": userRelays,
					"relayCount": len(userRelays),
					"read":       mailboxes.Read,
					"write":      mailboxes.Write, // Include write relays
					"both":       mailboxes.Both,
				}
			}
		}
	}

	// Get user's client relays (managed relay list for this client)
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

	// Create comprehensive cache response (no session data)
	response := map[string]interface{}{
		"publicKey":      publicKey,
		"npub":           npub,
		"cacheTimestamp": cachedData.Timestamp,
		"cacheAge":       cachedData.Timestamp.Format("2006-01-02 15:04:05"),
		"refreshed":      !found,
	}

	// Add metadata
	if parsedMetadata != nil {
		response["metadata"] = parsedMetadata
	} else if cachedData.Metadata != "" {
		response["metadataRaw"] = cachedData.Metadata
	}

	// Add mailboxes
	if parsedMailboxes != nil {
		response["mailboxes"] = parsedMailboxes
	} else if cachedData.Mailboxes != "" {
		response["mailboxesRaw"] = cachedData.Mailboxes
	}

	// Add relays (from user's Nostr mailboxes)
	if relays != nil {
		response["relays"] = relays
	}

	// Add clientRelays (user's managed client relay list)
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
