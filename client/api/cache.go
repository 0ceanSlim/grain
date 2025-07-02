package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/data"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetCacheHandler returns the cached user data as JSON
// Automatically refreshes cache if expired or missing
func GetCacheHandler(w http.ResponseWriter, r *http.Request) {
	// Get current session using the  session manager
	session := session.SessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		log.Util().Debug("No active session found for cache request")
		return
	}

	publicKey := session.PublicKey
	cachedData, found := cache.GetUserData(publicKey)
	
	// If cache is missing or expired, try to refresh it
	if !found {
		log.Util().Info("Cache miss or expired, attempting refresh", "pubkey", publicKey)
		
		// Try to fetch fresh data in the background
		if err := data.FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
			log.Util().Error("Failed to refresh cache", "pubkey", publicKey, "error", err)
			
			// Return error response with refresh suggestion
			response := map[string]interface{}{
				"error":           "Cache expired and refresh failed",
				"message":         "Please try refreshing the page or logging in again",
				"pubkey":          publicKey,
				"refresh_failed":  true,
				"session_active":  true,
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(response)
			return
		}
		
		// Try to get cached data again after refresh
		cachedData, found = cache.GetUserData(publicKey)
		if !found {
			log.Util().Error("Cache still empty after refresh", "pubkey", publicKey)
			http.Error(w, "Failed to load user data", http.StatusInternalServerError)
			return
		}
		
		log.Util().Info("Cache refreshed successfully", "pubkey", publicKey)
	}

	// Parse metadata to extract useful fields
	var parsedMetadata map[string]interface{}
	if err := json.Unmarshal([]byte(cachedData.Metadata), &parsedMetadata); err != nil {
		log.Util().Warn("Failed to parse cached metadata", "pubkey", publicKey, "error", err)
	}

	// Parse mailboxes for better response structure
	var parsedMailboxes map[string]interface{}
	if err := json.Unmarshal([]byte(cachedData.Mailboxes), &parsedMailboxes); err != nil {
		log.Util().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
	}

	// Encode npub for user-friendly display
	npub, err := tools.EncodePubkey(publicKey)
	if err != nil {
		log.Util().Error("Failed to encode npub", "pubkey", publicKey, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create comprehensive response structure
	response := map[string]interface{}{
		"publicKey":       publicKey,
		"npub":           npub,
		"cacheTimestamp": cachedData.Timestamp,
		"cacheAge":       cachedData.Timestamp.Format("2006-01-02 15:04:05"),
		"refreshed":      !found, // Indicate if data was refreshed
		// Include session information
		"sessionMode":     session.Mode,
		"signingMethod":   session.SigningMethod,
	}

	// Include parsed data if available, otherwise raw data
	if parsedMetadata != nil {
		response["metadata"] = parsedMetadata
	} else {
		response["metadataRaw"] = cachedData.Metadata
	}

	if parsedMailboxes != nil {
		response["mailboxes"] = parsedMailboxes
		
		// Also provide relay info in the same format as session handler
		var mailboxes core.Mailboxes
		if json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxes) == nil {
			userRelays := mailboxes.ToStringSlice()
			response["relayInfo"] = map[string]interface{}{
				"userRelays": userRelays,
				"relayCount": len(userRelays),
				"read":       mailboxes.Read,
				"both":       mailboxes.Both,
			}
			// Note: removed the redundant "write": mailboxes.Write that was showing as null
		}
	} else {
		response["mailboxesRaw"] = cachedData.Mailboxes
	}

	log.Util().Debug("Returning cached data with session info", 
		"pubkey", publicKey,
		"mode", session.Mode,
		"cache_age", cachedData.Timestamp.Format("15:04:05"))

	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode cached data", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to retrieve cached data", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Cached data with session info retrieved successfully", "pubkey", publicKey)
}

// RefreshCacheHandler manually refreshes cache for the current user
func RefreshCacheHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current session
	session := session.SessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}

	publicKey := session.PublicKey
	log.Util().Info("Manual cache refresh requested", "pubkey", publicKey)

	// Clear existing cache first
	cache.ClearUserData(publicKey)

	// Fetch fresh data
	if err := data.FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
		log.Util().Error("Manual cache refresh failed", "pubkey", publicKey, "error", err)
		
		response := map[string]interface{}{
			"success": false,
			"message": "Failed to refresh cache: " + err.Error(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Cache refreshed successfully",
		"pubkey":  publicKey,
	}

	log.Util().Info("Manual cache refresh successful", "pubkey", publicKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}