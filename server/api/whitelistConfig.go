package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// WhitelistConfigResponse represents the whitelist configuration response
type WhitelistConfigResponse struct {
	PubkeyWhitelist struct {
		Enabled             bool     `json:"enabled"`
		Pubkeys             []string `json:"pubkeys"`
		Npubs               []string `json:"npubs"`
		CacheRefreshMinutes int      `json:"cache_refresh_minutes"`
	} `json:"pubkey_whitelist"`
	KindWhitelist struct {
		Enabled bool     `json:"enabled"`
		Kinds   []string `json:"kinds"`
	} `json:"kind_whitelist"`
	DomainWhitelist struct {
		Enabled             bool     `json:"enabled"`
		Domains             []string `json:"domains"`
		CacheRefreshMinutes int      `json:"cache_refresh_minutes"`
	} `json:"domain_whitelist"`
}

// GetWhitelistConfig handles the request to return whitelist configuration
func GetWhitelistConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Whitelist config API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get the current whitelist configuration
	cfg := config.GetWhitelistConfig()
	if cfg == nil {
		log.RelayAPI().Error("Whitelist configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Whitelist configuration not available", http.StatusInternalServerError)
		return
	}

	// Prepare response with whitelist configuration
	response := WhitelistConfigResponse{
		PubkeyWhitelist: struct {
			Enabled             bool     `json:"enabled"`
			Pubkeys             []string `json:"pubkeys"`
			Npubs               []string `json:"npubs"`
			CacheRefreshMinutes int      `json:"cache_refresh_minutes"`
		}{
			Enabled:             cfg.PubkeyWhitelist.Enabled,
			Pubkeys:             cfg.PubkeyWhitelist.Pubkeys,
			Npubs:               cfg.PubkeyWhitelist.Npubs,
			CacheRefreshMinutes: cfg.PubkeyWhitelist.CacheRefreshMinutes,
		},
		KindWhitelist: struct {
			Enabled bool     `json:"enabled"`
			Kinds   []string `json:"kinds"`
		}{
			Enabled: cfg.KindWhitelist.Enabled,
			Kinds:   cfg.KindWhitelist.Kinds,
		},
		DomainWhitelist: struct {
			Enabled             bool     `json:"enabled"`
			Domains             []string `json:"domains"`
			CacheRefreshMinutes int      `json:"cache_refresh_minutes"`
		}{
			Enabled:             cfg.DomainWhitelist.Enabled,
			Domains:             cfg.DomainWhitelist.Domains,
			CacheRefreshMinutes: cfg.DomainWhitelist.CacheRefreshMinutes,
		},
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode whitelist config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Whitelist config served successfully",
		"client_ip", utils.GetClientIP(r))
}
