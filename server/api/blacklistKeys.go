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

// BlacklistKeysResponse represents the blacklist keys response
type BlacklistKeysResponse struct {
	Permanent []string                   `json:"permanent"`
	Temporary []map[string]interface{}   `json:"temporary"`
	Mutelist  map[string][]string        `json:"mutelist"`
}

// GetAllBlacklistedPubkeys handles the request to return all blacklisted pubkeys
func GetAllBlacklistedPubkeys(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Blacklist keys API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get blacklist configuration (regardless of enabled state)
	blacklistConfig := config.GetBlacklistConfig()
	if blacklistConfig == nil {
		log.RelayAPI().Error("Blacklist configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Blacklist configuration not available", http.StatusInternalServerError)
		return
	}

	// Convert npubs to hex pubkeys for permanent list
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

	// Add hex-format permanent pubkeys
	permanent = append(permanent, blacklistConfig.PermanentBlacklistPubkeys...)

	// Get temporary blacklisted pubkeys with expiration times
	temporary := config.GetTemporaryBlacklist()

	// Fetch mutelist pubkeys grouped by author
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

		// Fetch mutelist for each author
		for _, authorPubkey := range blacklistConfig.MuteListAuthors {
			mutelistedPubkeys, err := config.FetchPubkeysFromLocalMuteList(localRelayURL, []string{authorPubkey})
			if err != nil {
				log.RelayAPI().Error("Failed to fetch mutelist",
					"author", authorPubkey,
					"error", err)
				continue
			}

			if len(mutelistedPubkeys) > 0 {
				mutelist[authorPubkey] = mutelistedPubkeys
			}
		}
	}

	// Prepare response
	response := BlacklistKeysResponse{
		Permanent: permanent,
		Temporary: temporary,
		Mutelist:  mutelist,
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
		"permanent_count", len(permanent),
		"temporary_count", len(temporary),
		"mutelist_authors", len(mutelist))
}