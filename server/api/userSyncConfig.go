package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// UserSyncConfigResponse represents the user sync configuration response
type UserSyncConfigResponse struct {
	UserSync              bool     `json:"user_sync"`
	DisableAtStartup      bool     `json:"disable_at_startup"`
	InitialSyncRelays     []string `json:"initial_sync_relays"`
	Kinds                 []int    `json:"kinds"`
	Categories            string   `json:"categories"`
	Limit                 *int     `json:"limit"`
	ExcludeNonWhitelisted bool     `json:"exclude_non_whitelisted"`
	Interval              int      `json:"interval"`
}

// GetUserSyncConfig handles the request to return user sync configuration
func GetUserSyncConfig(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("User sync config API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get the current server configuration
	cfg := config.GetConfig()
	if cfg == nil {
		log.Util().Error("Server configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Server configuration not available", http.StatusInternalServerError)
		return
	}

	// Prepare response with user sync configuration
	response := UserSyncConfigResponse{
		UserSync:              cfg.UserSync.UserSync,
		DisableAtStartup:      cfg.UserSync.DisableAtStartup,
		InitialSyncRelays:     cfg.UserSync.InitialSyncRelays,
		Kinds:                 cfg.UserSync.Kinds,
		Categories:            cfg.UserSync.Categories,
		Limit:                 cfg.UserSync.Limit,
		ExcludeNonWhitelisted: cfg.UserSync.ExcludeNonWhitelisted,
		Interval:              cfg.UserSync.Interval,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode user sync config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.Util().Info("User sync config served successfully",
		"client_ip", utils.GetClientIP(r))
}