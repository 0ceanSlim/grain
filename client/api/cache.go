package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetCacheHandler returns the cached user data as JSON
func GetCacheHandler(w http.ResponseWriter, r *http.Request) {
	// Get current session using the session manager
	session := auth.SessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		log.Util().Debug("No active session found for cache request")
		return
	}

	publicKey := session.PublicKey
	cachedData, found := cache.GetUserData(publicKey)
	if !found {
		http.Error(w, "No cached data found", http.StatusNotFound)
		log.Util().Warn("No cached data found", "pubkey", publicKey)
		return
	}

	// Parse metadata to extract useful fields for better response structure
	var parsedMetadata map[string]interface{}
	if err := json.Unmarshal([]byte(cachedData.Metadata), &parsedMetadata); err != nil {
		log.Util().Warn("Failed to parse cached metadata", "pubkey", publicKey, "error", err)
		// Continue with raw metadata if parsing fails
	}

	// Parse mailboxes for better response structure
	var parsedMailboxes map[string]interface{}
	if err := json.Unmarshal([]byte(cachedData.Mailboxes), &parsedMailboxes); err != nil {
		log.Util().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
		// Continue with raw mailboxes if parsing fails
	}

	// Encode npub for user-friendly display
	npub, err := utils.EncodeNpub(publicKey)
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
	}

	// Include parsed data if available, otherwise raw data
	if parsedMetadata != nil {
		response["metadata"] = parsedMetadata
	} else {
		response["metadataRaw"] = cachedData.Metadata
	}

	if parsedMailboxes != nil {
		response["mailboxes"] = parsedMailboxes
	} else {
		response["mailboxesRaw"] = cachedData.Mailboxes
	}

	log.Util().Debug("Returning cached data", 
		"pubkey", publicKey,
		"cache_age", cachedData.Timestamp.Format("15:04:05"))

	// Set JSON response headers
	w.Header().Set("Content-Type", "application/json")
	
	// Encode cached data as JSON and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode cached data", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to retrieve cached data", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Cached data retrieved successfully", "pubkey", publicKey)
}