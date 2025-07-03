package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// EventTimeConstraintsConfigResponse represents the event time constraints configuration response
type EventTimeConstraintsConfigResponse struct {
	MinCreatedAt       int64  `json:"min_created_at"`
	MinCreatedAtString string `json:"min_created_at_string"`
	MaxCreatedAt       int64  `json:"max_created_at"`
	MaxCreatedAtString string `json:"max_created_at_string"`
}

// GetEventTimeConstraintsConfig handles the request to return event time constraints configuration
func GetEventTimeConstraintsConfig(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Event time constraints config API endpoint accessed",
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

	// Prepare response with event time constraints configuration
	response := EventTimeConstraintsConfigResponse{
		MinCreatedAt:       cfg.EventTimeConstraints.MinCreatedAt,
		MinCreatedAtString: cfg.EventTimeConstraints.MinCreatedAtString,
		MaxCreatedAt:       cfg.EventTimeConstraints.MaxCreatedAt,
		MaxCreatedAtString: cfg.EventTimeConstraints.MaxCreatedAtString,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode event time constraints config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Event time constraints config served successfully",
		"client_ip", utils.GetClientIP(r))
}