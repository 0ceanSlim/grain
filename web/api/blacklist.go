package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
)

// GetAllBlacklistedPubkeys returns a full list of blacklisted pubkeys, including mutelist authors
func GetAllBlacklistedPubkeys(w http.ResponseWriter, r *http.Request) {
	blacklistConfig := config.GetBlacklistConfig()
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		http.Error(w, "Blacklist is not enabled", http.StatusNotFound)
		return
	}

	// Convert npubs in PermanentBlacklistNpubs to hex pubkeys
	var permanent []string
	for _, npub := range blacklistConfig.PermanentBlacklistNpubs {
		decodedPubKey, err := utils.DecodeNpub(npub)
		if err != nil {
			log.Printf("Error decoding npub %s: %v", npub, err)
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
			log.Println("Server configuration is not loaded")
			http.Error(w, "Internal server error: server configuration is missing", http.StatusInternalServerError)
			return
		}

		localRelayURL := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)

		// Loop through each mutelist author and fetch their mutelisted pubkeys
		for _, authorPubkey := range blacklistConfig.MuteListAuthors {
			mutelistedPubkeys, err := config.FetchPubkeysFromLocalMuteList(localRelayURL, []string{authorPubkey})
			if err != nil {
				log.Printf("Error fetching pubkeys from mutelist author %s: %v", authorPubkey, err)
				continue
			}

			if len(mutelistedPubkeys) > 0 {
				mutelist[authorPubkey] = mutelistedPubkeys
			}
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"permanent": permanent,
		"temporary": temporary, // Now includes expiration timestamps
		"mutelist":  mutelist,  // Grouped by author
	}

	// Encode JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
