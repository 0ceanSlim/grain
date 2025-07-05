package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
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

// GetAllWhitelistedPubkeys handles the request to return all whitelisted pubkeys organized by source
func GetAllWhitelistedPubkeys(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Whitelist keys API endpoint accessed",
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

	// Collect all pubkeys from config (regardless of enabled state)
	var listPubkeys []string

	// Add direct pubkeys from config
	listPubkeys = append(listPubkeys, cfg.PubkeyWhitelist.Pubkeys...)

	// Convert npubs to pubkeys and add them
	for _, npub := range cfg.PubkeyWhitelist.Npubs {
		decodedPubKey, err := tools.DecodeNpub(npub)
		if err != nil {
			log.RelayAPI().Error("Failed to decode npub",
				"npub", npub,
				"error", err)
			continue
		}
		listPubkeys = append(listPubkeys, decodedPubKey)
	}

	// Fetch pubkeys from domains (regardless of enabled state)
	var domainInfos []WhitelistDomainInfo
	for _, domain := range cfg.DomainWhitelist.Domains {
		domainPubkeys, err := utils.FetchPubkeysFromDomains([]string{domain})
		if err != nil {
			log.RelayAPI().Error("Failed to fetch pubkeys from domain",
				"domain", domain,
				"error", err)
			// Still include the domain but with empty pubkeys array
			domainInfos = append(domainInfos, WhitelistDomainInfo{
				Domain:  domain,
				Pubkeys: []string{},
			})
			continue
		}

		domainInfos = append(domainInfos, WhitelistDomainInfo{
			Domain:  domain,
			Pubkeys: domainPubkeys,
		})
	}

	// Prepare response
	response := WhitelistKeysResponse{
		List:    listPubkeys,
		Domains: domainInfos,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode whitelist keys response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Whitelist keys served successfully",
		"client_ip", utils.GetClientIP(r),
		"list_pubkeys", len(listPubkeys),
		"domain_count", len(domainInfos))
}