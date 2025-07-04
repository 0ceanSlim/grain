package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ServerConfigResponse represents the server configuration response
type ServerConfigResponse struct {
	Port                      string `json:"port"`
	ReadTimeout               int    `json:"read_timeout"`
	WriteTimeout              int    `json:"write_timeout"`
	IdleTimeout               int    `json:"idle_timeout"`
	MaxSubscriptionsPerClient int    `json:"max_subscriptions_per_client"`
	ImplicitReqLimit          int    `json:"implicit_req_limit"`
}

// GetServerConfig handles the request to return server configuration
func GetServerConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("Server config API endpoint accessed",
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

	// Prepare response with server configuration
	response := ServerConfigResponse{
		Port:                      cfg.Server.Port,
		ReadTimeout:               cfg.Server.ReadTimeout,
		WriteTimeout:              cfg.Server.WriteTimeout,
		IdleTimeout:               cfg.Server.IdleTimeout,
		MaxSubscriptionsPerClient: cfg.Server.MaxSubscriptionsPerClient,
		ImplicitReqLimit:          cfg.Server.ImplicitReqLimit,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode server config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Server config served successfully",
		"client_ip", utils.GetClientIP(r))
}