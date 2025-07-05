package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// BlacklistConfigResponse represents the blacklist configuration response
type BlacklistConfigResponse struct {
	Enabled                     bool     `json:"enabled"`
	PermanentBanWords           []string `json:"permanent_ban_words"`
	TempBanWords                []string `json:"temp_ban_words"`
	MaxTempBans                 int      `json:"max_temp_bans"`
	TempBanDuration             int      `json:"temp_ban_duration"`
	PermanentBlacklistPubkeys   []string `json:"permanent_blacklist_pubkeys"`
	PermanentBlacklistNpubs     []string `json:"permanent_blacklist_npubs"`
	MuteListAuthors             []string `json:"mutelist_authors"`
	MutelistCacheRefreshMinutes int      `json:"mutelist_cache_refresh_minutes"`
}

// GetBlacklistConfig handles the request to return blacklist configuration
func GetBlacklistConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Blacklist config API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get the current blacklist configuration
	cfg := config.GetBlacklistConfig()
	if cfg == nil {
		log.RelayAPI().Error("Blacklist configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Blacklist configuration not available", http.StatusInternalServerError)
		return
	}

	// Prepare response with blacklist configuration
	response := BlacklistConfigResponse{
		Enabled:                     cfg.Enabled,
		PermanentBanWords:           cfg.PermanentBanWords,
		TempBanWords:                cfg.TempBanWords,
		MaxTempBans:                 cfg.MaxTempBans,
		TempBanDuration:             cfg.TempBanDuration,
		PermanentBlacklistPubkeys:   cfg.PermanentBlacklistPubkeys,
		PermanentBlacklistNpubs:     cfg.PermanentBlacklistNpubs,
		MuteListAuthors:             cfg.MuteListAuthors,
		MutelistCacheRefreshMinutes: cfg.MutelistCacheRefreshMinutes,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode blacklist config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Blacklist config served successfully",
		"client_ip", utils.GetClientIP(r))
}