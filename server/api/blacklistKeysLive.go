package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetAllBlacklistedPubkeysLive handles the request to return all blacklisted pubkeys with live mutelist fetching
// This endpoint fetches fresh data from mutelists and is suitable for verification after configuration changes
func GetAllBlacklistedPubkeysLive(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Live blacklist keys API endpoint accessed",
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

	// Build permanent list from config
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

	// Fetch mutelist pubkeys grouped by author LIVE
	mutelist := make(map[string][]string)

	if len(blacklistConfig.MuteListAuthors) > 0 {
		cfg := config.GetConfig()
		if cfg == nil {
			log.RelayAPI().Error("Server configuration not loaded",
				"client_ip", utils.GetClientIP(r))
			http.Error(w, "Server configuration not available", http.StatusInternalServerError)
			return
		}

		localRelayURL := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)

		// Fetch mutelist for each author LIVE
		for _, authorPubkey := range blacklistConfig.MuteListAuthors {
			mutelistedPubkeys, err := config.FetchPubkeysFromLocalMuteList(localRelayURL, []string{authorPubkey})
			if err != nil {
				log.RelayAPI().Error("Failed to fetch live mutelist",
					"author", authorPubkey,
					"error", err)
				continue
			}

			if len(mutelistedPubkeys) > 0 {
				mutelist[authorPubkey] = mutelistedPubkeys
			}
		}
	}

	// Build complete list for response
	var allPubkeys []string
	allPubkeys = append(allPubkeys, permanent...)

	// Add temporary pubkeys to complete list
	for _, tempEntry := range temporary {
		if pubkey, ok := tempEntry["pubkey"].(string); ok {
			allPubkeys = append(allPubkeys, pubkey)
		}
	}

	// Add mutelist pubkeys to complete list
	for _, authorPubkeys := range mutelist {
		allPubkeys = append(allPubkeys, authorPubkeys...)
	}

	// Prepare response
	response := BlacklistKeysResponse{
		List:      allPubkeys,
		Permanent: permanent,
		Temporary: temporary,
		Mutelist:  flattenMutelist(mutelist),
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode live blacklist keys response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Live blacklist keys served successfully",
		"client_ip", utils.GetClientIP(r),
		"live_pubkeys", len(allPubkeys),
		"permanent_count", len(permanent),
		"temporary_count", len(temporary),
		"mutelist_authors", len(mutelist))
}

// flattenMutelist converts grouped mutelist to flat array for consistency with cached endpoint
func flattenMutelist(mutelist map[string][]string) []string {
	var flattened []string
	for _, authorPubkeys := range mutelist {
		flattened = append(flattened, authorPubkeys...)
	}
	return flattened
}