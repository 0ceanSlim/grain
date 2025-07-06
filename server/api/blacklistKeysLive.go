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
	var mutelist map[string][]string
	if len(blacklistConfig.MuteListAuthors) > 0 {
		cfg := config.GetConfig()
		if cfg == nil {
			log.RelayAPI().Error("Server configuration not loaded",
				"client_ip", utils.GetClientIP(r))
			http.Error(w, "Server configuration not available", http.StatusInternalServerError)
			return
		}

		localRelayURL := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)

		var err error
		mutelist, err = config.FetchGroupedMuteListPubkeys(localRelayURL, blacklistConfig.MuteListAuthors)
		if err != nil {
			log.RelayAPI().Error("Failed to fetch grouped mutelist",
				"error", err)
			mutelist = make(map[string][]string) // Empty map on error
		}
	} else {
		mutelist = make(map[string][]string)
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
		log.RelayAPI().Error("Failed to encode live blacklist keys response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Live blacklist keys served successfully",
		"client_ip", utils.GetClientIP(r),
		"permanent_count", len(permanent),
		"temporary_count", len(temporary),
		"mutelist_authors", len(mutelist))
}