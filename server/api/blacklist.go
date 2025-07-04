package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetAllBlacklistedPubkeys returns a full list of blacklisted pubkeys, including mutelist authors
func GetAllBlacklistedPubkeys(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger("RelayAPI")
	
	blacklistConfig := config.GetBlacklistConfig()
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		logger.Warn("Blacklist access attempted but not enabled")
		http.Error(w, "Blacklist is not enabled", http.StatusNotFound)
		return
	}

	logger.Debug("Processing blacklist request")

	// Convert npubs in PermanentBlacklistNpubs to hex pubkeys
	var permanent []string
	for _, npub := range blacklistConfig.PermanentBlacklistNpubs {
		decodedPubKey, err := tools.DecodeNpub(npub)
		if err != nil {
			logger.Error("Failed to decode npub",
				"npub", npub,
				"error", err)
			continue
		}
		permanent = append(permanent, decodedPubKey)
	}

	// Include already hex-format permanent pubkeys
	permanent = append(permanent, blacklistConfig.PermanentBlacklistPubkeys...)

	// Fetch temporary blacklisted pubkeys with expiration times
	temporary := config.GetTemporaryBlacklist()

	// Fetch mutelist pubkeys grouped by author
	mutelist := make(map[string][]string) // key: author, value: list of pubkeys

	if len(blacklistConfig.MuteListAuthors) > 0 {
		cfg := config.GetConfig()
		if cfg == nil {
			logger.Error("Server configuration not loaded")
			http.Error(w, "Internal server error: server configuration is missing", http.StatusInternalServerError)
			return
		}

		localRelayURL := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)

		// Loop through each mutelist author and fetch their mutelisted pubkeys
		for _, authorPubkey := range blacklistConfig.MuteListAuthors {
			mutelistedPubkeys, err := config.FetchPubkeysFromLocalMuteList(localRelayURL, []string{authorPubkey})
			if err != nil {
				logger.Error("Failed to fetch mutelist",
					"author", authorPubkey,
					"error", err)
				continue
			}

			if len(mutelistedPubkeys) > 0 {
				mutelist[authorPubkey] = mutelistedPubkeys
				logger.Debug("Retrieved mutelist",
					"author", authorPubkey,
					"count", len(mutelistedPubkeys))
			}
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"permanent": permanent,
		"temporary": temporary, // Now includes expiration timestamps
		"mutelist":  mutelist,  // Grouped by author
	}

	logger.Info("Blacklist data retrieved",
		"permanent_count", len(permanent),
		"temporary_count", len(temporary),
		"mutelist_authors", len(mutelist))

	// Encode JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode blacklist response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}