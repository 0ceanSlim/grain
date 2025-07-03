package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// RateLimitConfigResponse represents the rate limiting configuration response
type RateLimitConfigResponse struct {
	WsLimit        float64                                `json:"ws_limit"`
	WsBurst        int                                    `json:"ws_burst"`
	EventLimit     float64                                `json:"event_limit"`
	EventBurst     int                                    `json:"event_burst"`
	ReqLimit       float64                                `json:"req_limit"`
	ReqBurst       int                                    `json:"req_burst"`
	MaxEventSize   int                                    `json:"max_event_size"`
	KindSizeLimits []cfgType.KindSizeLimitConfig          `json:"kind_size_limits"`
	CategoryLimits map[string]cfgType.KindLimitConfig     `json:"category_limits"`
	KindLimits     []cfgType.KindLimitConfig              `json:"kind_limits"`
}

// GetRateLimitConfig handles the request to return rate limiting configuration
func GetRateLimitConfig(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Rate limit config API endpoint accessed",
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

	// Prepare response with rate limiting configuration
	response := RateLimitConfigResponse{
		WsLimit:        cfg.RateLimit.WsLimit,
		WsBurst:        cfg.RateLimit.WsBurst,
		EventLimit:     cfg.RateLimit.EventLimit,
		EventBurst:     cfg.RateLimit.EventBurst,
		ReqLimit:       cfg.RateLimit.ReqLimit,
		ReqBurst:       cfg.RateLimit.ReqBurst,
		MaxEventSize:   cfg.RateLimit.MaxEventSize,
		KindSizeLimits: cfg.RateLimit.KindSizeLimits,
		CategoryLimits: cfg.RateLimit.CategoryLimits,
		KindLimits:     cfg.RateLimit.KindLimits,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode rate limit config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Rate limit config served successfully",
		"client_ip", utils.GetClientIP(r))
}