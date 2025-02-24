package api

import (
	"encoding/json"
	"log"
	"net/http"

	"grain/config"
	"grain/server/utils"
)

// GetAllWhitelistedPubkeys handles the request to return all whitelisted pubkeys
func GetAllWhitelistedPubkeys(w http.ResponseWriter, r *http.Request) {
	// Load whitelist configuration
	cfg := config.GetWhitelistConfig()
	if cfg == nil {
		http.Error(w, "Failed to load whitelist configuration", http.StatusInternalServerError)
		return
	}

	// Collect all whitelisted pubkeys
	whitelistedPubkeys := cfg.PubkeyWhitelist.Pubkeys

	// Convert npubs to pubkeys
	for _, npub := range cfg.PubkeyWhitelist.Npubs {
		decodedPubKey, err := utils.DecodeNpub(npub)
		if err != nil {
			log.Printf("Error decoding npub %s: %v", npub, err)
			continue
		}
		whitelistedPubkeys = append(whitelistedPubkeys, decodedPubKey)
	}

	// Always fetch pubkeys from domains, even if domain whitelisting is disabled
	domainPubkeys, err := utils.FetchPubkeysFromDomains(cfg.DomainWhitelist.Domains)
	if err != nil {
		log.Printf("Error fetching pubkeys from domains: %v", err)
	} else {
		whitelistedPubkeys = append(whitelistedPubkeys, domainPubkeys...)
	}

	// Prepare response
	response := map[string]interface{}{
		"pubkeys": whitelistedPubkeys,
	}

	// Encode JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
