package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetAllWhitelistedPubkeys handles the request to return all whitelisted pubkeys
func GetAllWhitelistedPubkeys(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger("RelayAPI")

	// Load whitelist configuration
	cfg := config.GetWhitelistConfig()
	if cfg == nil {
		logger.Error("Failed to load whitelist configuration")
		http.Error(w, "Failed to load whitelist configuration", http.StatusInternalServerError)
		return
	}

	logger.Debug("Processing whitelist request")

	// Collect all whitelisted pubkeys
	whitelistedPubkeys := cfg.PubkeyWhitelist.Pubkeys

	// Convert npubs to pubkeys
	for _, npub := range cfg.PubkeyWhitelist.Npubs {
		decodedPubKey, err := tools.DecodeNpub(npub)
		if err != nil {
			logger.Error("Failed to decode npub",
				"npub", npub,
				"error", err)
			continue
		}
		whitelistedPubkeys = append(whitelistedPubkeys, decodedPubKey)
	}

	// Always fetch pubkeys from domains, even if domain whitelisting is disabled
	domainPubkeys, err := utils.FetchPubkeysFromDomains(cfg.DomainWhitelist.Domains)
	if err != nil {
		logger.Error("Failed to fetch domain pubkeys", "error", err)
	} else {
		whitelistedPubkeys = append(whitelistedPubkeys, domainPubkeys...)
		logger.Debug("Retrieved domain pubkeys",
			"domain_count", len(cfg.DomainWhitelist.Domains),
			"pubkey_count", len(domainPubkeys))
	}

	// Prepare response
	response := map[string]interface{}{
		"pubkeys": whitelistedPubkeys,
	}

	logger.Info("Whitelist data retrieved",
		"total_pubkeys", len(whitelistedPubkeys),
		"direct_pubkeys", len(cfg.PubkeyWhitelist.Pubkeys),
		"npub_count", len(cfg.PubkeyWhitelist.Npubs),
		"domain_count", len(cfg.DomainWhitelist.Domains))

	// Encode JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode whitelist response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}