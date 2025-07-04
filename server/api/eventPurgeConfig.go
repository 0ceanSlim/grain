package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// EventPurgeConfigResponse represents the event purging configuration response
type EventPurgeConfigResponse struct {
	Enabled              bool            `json:"enabled"`
	DisableAtStartup     bool            `json:"disable_at_startup"`
	KeepIntervalHours    int             `json:"keep_interval_hours"`
	PurgeIntervalMinutes int             `json:"purge_interval_minutes"`
	PurgeByCategory      map[string]bool `json:"purge_by_category"`
	PurgeByKindEnabled   bool            `json:"purge_by_kind_enabled"`
	KindsToPurge         []int           `json:"kinds_to_purge"`
	ExcludeWhitelisted   bool            `json:"exclude_whitelisted"`
}

// GetEventPurgeConfig handles the request to return event purging configuration
func GetEventPurgeConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Event purge config API endpoint accessed",
		"client_ip", utils.GetClientIP(r),
		"user_agent", r.UserAgent())

	// Get the current server configuration
	cfg := config.GetConfig()
	if cfg == nil {
		log.RelayAPI().Error("Server configuration not loaded",
			"client_ip", utils.GetClientIP(r))
		http.Error(w, "Server configuration not available", http.StatusInternalServerError)
		return
	}

	// Prepare response with event purging configuration
	response := EventPurgeConfigResponse{
		Enabled:              cfg.EventPurge.Enabled,
		DisableAtStartup:     cfg.EventPurge.DisableAtStartup,
		KeepIntervalHours:    cfg.EventPurge.KeepIntervalHours,
		PurgeIntervalMinutes: cfg.EventPurge.PurgeIntervalMinutes,
		PurgeByCategory:      cfg.EventPurge.PurgeByCategory,
		PurgeByKindEnabled:   cfg.EventPurge.PurgeByKindEnabled,
		KindsToPurge:         cfg.EventPurge.KindsToPurge,
		ExcludeWhitelisted:   cfg.EventPurge.ExcludeWhitelisted,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode event purge config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Event purge config served successfully",
		"client_ip", utils.GetClientIP(r))
}
