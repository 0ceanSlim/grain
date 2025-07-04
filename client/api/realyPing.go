package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// RelayPingHandler pings a relay and returns response time and connection status
func RelayPingHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		log.ClientAPI().Warn("Invalid HTTP method for relay ping", 
			"method", r.Method, 
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get relay URL from query parameters
	relayURL := r.URL.Query().Get("url")
	if relayURL == "" {
		log.ClientAPI().Warn("Missing relay URL parameter", 
			"client_ip", r.RemoteAddr)
		http.Error(w, "Missing relay URL parameter", http.StatusBadRequest)
		return
	}

	// Validate URL format
	if !strings.HasPrefix(relayURL, "ws://") && !strings.HasPrefix(relayURL, "wss://") {
		log.ClientAPI().Warn("Invalid relay URL format", 
			"url", relayURL, 
			"client_ip", r.RemoteAddr)
		http.Error(w, "Invalid relay URL format", http.StatusBadRequest)
		return
	}

	log.ClientAPI().Debug("Starting relay ping", 
		"relay_url", relayURL, 
		"client_ip", r.RemoteAddr)

	// Ping the relay and measure response time
	startTime := time.Now()
	success := false
	var errorMsg string

	// Parse the relay URL
	_, err := url.Parse(relayURL)
	if err != nil {
		errorMsg = "invalid URL format"
		log.ClientAPI().Debug("Failed to parse relay URL", 
			"relay_url", relayURL, 
			"error", err)
	} else {
		// Create WebSocket connection using golang.org/x/net/websocket
		origin := "http://localhost/"
		
		// Create connection with timeout
		done := make(chan error, 1)
		var conn *websocket.Conn
		
		go func() {
			var dialErr error
			conn, dialErr = websocket.Dial(relayURL, "", origin)
			done <- dialErr
		}()
		
		// Wait for connection with timeout
		select {
		case err := <-done:
			if err != nil {
				errorMsg = err.Error()
				log.ClientAPI().Debug("Relay ping failed", 
					"relay_url", relayURL, 
					"error", err,
					"duration_ms", time.Since(startTime).Milliseconds())
			} else {
				success = true
				if conn != nil {
					conn.Close() // Close immediately after successful connection
				}
				log.ClientAPI().Debug("Relay ping successful", 
					"relay_url", relayURL, 
					"duration_ms", time.Since(startTime).Milliseconds())
			}
		case <-time.After(5 * time.Second):
			errorMsg = "connection timeout"
			log.ClientAPI().Debug("Relay ping timeout", 
				"relay_url", relayURL, 
				"timeout_seconds", 5,
				"duration_ms", time.Since(startTime).Milliseconds())
		}
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Prepare response
	response := map[string]interface{}{
		"success":      success,
		"responseTime": responseTime,
		"relay":        relayURL,
	}

	if !success {
		response["error"] = errorMsg
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	
	// Return response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode ping response", 
			"error", err, 
			"relay_url", relayURL,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Relay ping completed", 
		"relay_url", relayURL, 
		"success", success, 
		"response_time_ms", responseTime,
		"client_ip", r.RemoteAddr)
}