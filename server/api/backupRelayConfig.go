package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// BackupRelayConfigResponse represents the backup relay configuration response
type BackupRelayConfigResponse struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

// GetBackupRelayConfig handles the request to return backup relay configuration
func GetBackupRelayConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Backup relay config API endpoint accessed",
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

	// Prepare response with backup relay configuration
	response := BackupRelayConfigResponse{
		Enabled: cfg.BackupRelay.Enabled,
		URL:     cfg.BackupRelay.URL,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode backup relay config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Backup relay config served successfully",
		"client_ip", utils.GetClientIP(r))
}
