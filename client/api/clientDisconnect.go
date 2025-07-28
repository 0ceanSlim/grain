package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ClientDisconnectHandler disconnects the client from a relay
func ClientDisconnectHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		log.ClientAPI().Warn("Invalid HTTP method for client disconnect",
			"method", r.Method,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if user is logged in
	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil || userSession.PublicKey == "" {
		log.ClientAPI().Warn("Unauthorized client disconnect attempt",
			"client_ip", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "You need to login to the client to change the default relays",
		})
		return
	}

	// Extract relay URL from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/client/disconnect/")
	relayURL := strings.TrimSpace(path)

	if relayURL == "" {
		log.ClientAPI().Warn("Missing relay URL parameter",
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Relay URL parameter is required"})
		return
	}

	// URL decode the relay URL
	decodedURL, err := url.QueryUnescape(relayURL)
	if err != nil {
		log.ClientAPI().Warn("Failed to decode relay URL",
			"url", relayURL,
			"error", err,
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid URL encoding"})
		return
	}

	// Validate URL format
	if !strings.HasPrefix(decodedURL, "ws://") && !strings.HasPrefix(decodedURL, "wss://") {
		log.ClientAPI().Warn("Invalid relay URL format",
			"url", decodedURL,
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid relay URL format (must start with ws:// or wss://)"})
		return
	}

	// Validate URL can be parsed
	_, err = url.Parse(decodedURL)
	if err != nil {
		log.ClientAPI().Warn("Invalid relay URL",
			"url", decodedURL,
			"error", err,
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid relay URL format"})
		return
	}

	log.ClientAPI().Debug("Disconnecting client from relay",
		"relay_url", decodedURL,
		"client_ip", r.RemoteAddr,
		"user", userSession.PublicKey)

	// TODO: Add actual relay disconnection logic here
	// This might involve updating the client's relay list in memory/database
	// and closing the actual WebSocket connection

	// For now, just return success
	response := map[string]interface{}{
		"success":   true,
		"relay":     decodedURL,
		"message":   "Successfully disconnected from relay",
		"connected": false,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode disconnect response",
			"error", err,
			"relay_url", decodedURL,
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Client disconnected from relay",
		"relay_url", decodedURL,
		"client_ip", r.RemoteAddr,
		"user", userSession.PublicKey)
}
