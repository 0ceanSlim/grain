package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ResourceLimitsConfigResponse represents the resource limits configuration response
type ResourceLimitsConfigResponse struct {
	CPUCores   int `json:"cpu_cores"`
	MemoryMB   int `json:"memory_mb"`
	HeapSizeMB int `json:"heap_size_mb"`
}

// GetResourceLimitsConfig handles the request to return resource limits configuration
func GetResourceLimitsConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Resource limits config API endpoint accessed",
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

	// Prepare response with resource limits configuration
	response := ResourceLimitsConfigResponse{
		CPUCores:   cfg.ResourceLimits.CPUCores,
		MemoryMB:   cfg.ResourceLimits.MemoryMB,
		HeapSizeMB: cfg.ResourceLimits.HeapSizeMB,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode resource limits config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Resource limits config served successfully",
		"client_ip", utils.GetClientIP(r))
}
