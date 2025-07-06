package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// BlacklistKeysResponse represents the blacklist keys response
type BlacklistKeysResponse struct {
	List      []string                 `json:"list"`
	Permanent []string                 `json:"permanent"`
	Temporary []map[string]interface{} `json:"temporary"`
	Mutelist  []string                 `json:"mutelist"`
}

// GetAllBlacklistedPubkeys handles the request to return all blacklisted pubkeys organized by source
func GetAllBlacklistedPubkeys(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Blacklist keys API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get blacklist configuration
	blacklistConfig := config.GetBlacklistConfig()
	if blacklistConfig == nil {
		log.RelayAPI().Error("Blacklist configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Blacklist configuration not available", http.StatusInternalServerError)
		return
	}

	// Get cached blacklist pubkeys - this includes all sources (permanent + temporary + mutelist)
	pubkeyCache := config.GetPubkeyCache()
	cachedBlacklistPubkeys := pubkeyCache.GetBlacklistedPubkeys()

	log.RelayAPI().Debug("Retrieved cached blacklist pubkeys",
		"cached_count", len(cachedBlacklistPubkeys))

	// Build permanent list from config (for breakdown)
	var permanent []string
	for _, npub := range blacklistConfig.PermanentBlacklistNpubs {
		decodedPubKey, err := tools.DecodeNpub(npub)
		if err != nil {
			log.RelayAPI().Error("Failed to decode npub",
				"npub", npub,
				"error", err)
			continue
		}
		permanent = append(permanent, decodedPubKey)
	}
	permanent = append(permanent, blacklistConfig.PermanentBlacklistPubkeys...)

	// Get temporary blacklisted pubkeys with expiration times
	temporary := config.GetTemporaryBlacklist()

	// Extract mutelist pubkeys from cache by subtracting permanent and temporary
	mutelistFromCache := extractMutelistFromCache(cachedBlacklistPubkeys, permanent, temporary)

	// Prepare response using cached data for list
	response := BlacklistKeysResponse{
		List:      cachedBlacklistPubkeys,
		Permanent: permanent,
		Temporary: temporary,
		Mutelist:  mutelistFromCache,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode blacklist keys response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Blacklist keys served successfully",
		"client_ip", utils.GetClientIP(r),
		"cached_pubkeys", len(cachedBlacklistPubkeys),
		"permanent_count", len(permanent),
		"temporary_count", len(temporary),
		"mutelist_count", len(mutelistFromCache))
}

// extractMutelistFromCache extracts mutelist pubkeys by filtering out permanent and temporary ones
func extractMutelistFromCache(allCached, permanent []string, temporary []map[string]interface{}) []string {
	// Create lookup maps for efficient filtering
	permanentMap := make(map[string]bool)
	for _, pubkey := range permanent {
		permanentMap[pubkey] = true
	}

	temporaryMap := make(map[string]bool)
	for _, tempEntry := range temporary {
		if pubkey, ok := tempEntry["pubkey"].(string); ok {
			temporaryMap[pubkey] = true
		}
	}

	// Extract mutelist by filtering out permanent and temporary
	var mutelist []string
	for _, pubkey := range allCached {
		if !permanentMap[pubkey] && !temporaryMap[pubkey] {
			mutelist = append(mutelist, pubkey)
		}
	}

	return mutelist
}