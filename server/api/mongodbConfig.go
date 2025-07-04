package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// MongoDBConfigResponse represents the MongoDB configuration response
type MongoDBConfigResponse struct {
	URI      string `json:"uri"`
	Database string `json:"database"`
}

// sanitizeMongoURI removes credentials from MongoDB URI for security
func sanitizeMongoURI(uri string) string {
	// Pattern to match MongoDB URIs with credentials
	credentialsPattern := regexp.MustCompile(`mongodb://[^:]+:[^@]+@`)
	
	// Replace credentials with [HIDDEN]
	sanitized := credentialsPattern.ReplaceAllString(uri, "mongodb://[HIDDEN]@")
	
	// Also handle mongodb+srv URIs
	credentialsPatternSRV := regexp.MustCompile(`mongodb\+srv://[^:]+:[^@]+@`)
	sanitized = credentialsPatternSRV.ReplaceAllString(sanitized, "mongodb+srv://[HIDDEN]@")
	
	return sanitized
}

// GetMongoDBConfig handles the request to return MongoDB configuration
func GetMongoDBConfig(w http.ResponseWriter, r *http.Request) {
	log.RelayAPI().Debug("MongoDB config API endpoint accessed",
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

	// Prepare response with sanitized MongoDB configuration
	response := MongoDBConfigResponse{
		URI:      sanitizeMongoURI(cfg.MongoDB.URI),
		Database: cfg.MongoDB.Database,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.RelayAPI().Error("Failed to encode MongoDB config response",
			"client_ip", utils.GetClientIP(r),
			"error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("MongoDB config served successfully",
		"client_ip", utils.GetClientIP(r))
}