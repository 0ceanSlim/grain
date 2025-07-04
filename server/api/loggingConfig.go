package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// LoggingConfigResponse represents the logging configuration response
type LoggingConfigResponse struct {
	Level              string   `json:"level"`
	File               string   `json:"file"`
	MaxSizeMB          int      `json:"max_log_size_mb"`
	Structure          bool     `json:"structure"`
	CheckIntervalMin   int      `json:"check_interval_min"`
	BackupCount        int      `json:"backup_count"`
	SuppressComponents []string `json:"suppress_components"`
}

// GetLoggingConfig handles the request to return logging configuration
func GetLoggingConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Logging config API endpoint accessed",
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

	// Prepare response with logging configuration
	response := LoggingConfigResponse{
		Level:              cfg.Logging.Level,
		File:               cfg.Logging.File,
		MaxSizeMB:          cfg.Logging.MaxSizeMB,
		Structure:          cfg.Logging.Structure,
		CheckIntervalMin:   cfg.Logging.CheckIntervalMin,
		BackupCount:        cfg.Logging.BackupCount,
		SuppressComponents: cfg.Logging.SuppressComponents,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode logging config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Logging config served successfully",
		"client_ip", utils.GetClientIP(r))
}
