package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// WhitelistKeysResponse represents the whitelist keys response
type WhitelistKeysResponse struct {
	List    []string                    `json:"list"`
	Domains []WhitelistDomainInfo       `json:"domains"`
}

// WhitelistDomainInfo represents domain information with its pubkeys
type WhitelistDomainInfo struct {
	Domain  string   `json:"domain"`
	Pubkeys []string `json:"pubkeys"`
}

// GetAllWhitelistedPubkeys handles the cached request to return all whitelisted pubkeys organized by source
func GetAllWhitelistedPubkeys(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Cached whitelist keys API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get whitelist configuration
	cfg := config.GetWhitelistConfig()
	if cfg == nil {
		log.RelayAPI().Error("Whitelist configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Whitelist configuration not available", http.StatusInternalServerError)
		return
	}

	// Get cached whitelist pubkeys - this includes all sources (config + domains)
	pubkeyCache := config.GetPubkeyCache()
	// Use GetDirectWhitelistedPubkeys() to get only direct config pubkeys (not domain pubkeys)
	// This avoids duplication since domain pubkeys are shown separately in domains section
	cachedPubkeys := pubkeyCache.GetDirectWhitelistedPubkeys()

	log.RelayAPI().Debug("Retrieved direct cached whitelist pubkeys",
		"direct_cached_count", len(cachedPubkeys))

	// Build domain breakdown using cached data
	var domainInfos []WhitelistDomainInfo
	for _, domain := range cfg.DomainWhitelist.Domains {
		// Get cached domain pubkeys (no live fetching!)
		domainPubkeys := pubkeyCache.GetDomainPubkeys(domain)
		
		domainInfos = append(domainInfos, WhitelistDomainInfo{
			Domain:  domain,
			Pubkeys: domainPubkeys,
		})

		log.RelayAPI().Debug("Added cached domain pubkeys to response",
			"domain", domain,
			"cached_pubkey_count", len(domainPubkeys))
	}

	// Prepare response using cached data for both list and domains
	response := WhitelistKeysResponse{
		List:    cachedPubkeys,
		Domains: domainInfos,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode cached whitelist keys response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Cached whitelist keys served successfully",
		"client_ip", utils.GetClientIP(r),
		"cached_pubkeys", len(cachedPubkeys),
		"domain_count", len(domainInfos))
}