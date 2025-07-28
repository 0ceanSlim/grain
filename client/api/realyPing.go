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

// PingHandler pings any relay and returns response time and connection status
func PingHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		log.ClientAPI().Warn("Invalid HTTP method for ping",
			"method", r.Method,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract relay URL from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ping/")
	relayURL := strings.TrimSpace(path)

	if relayURL == "" {
		log.ClientAPI().Warn("Missing relay URL parameter",
			"client_ip", r.RemoteAddr)
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
			"client_ip", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid URL encoding"})
		return
	}

	// Validate URL format
	if !strings.HasPrefix(decodedURL, "ws://") && !strings.HasPrefix(decodedURL, "wss://") {
		log.ClientAPI().Warn("Invalid relay URL format",
			"url", decodedURL,
			"client_ip", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid relay URL format (must start with ws:// or wss://)"})
		return
	}

	log.ClientAPI().Debug("Starting relay ping",
		"relay_url", decodedURL,
		"client_ip", r.RemoteAddr)

	// Ping the relay and measure response time
	startTime := time.Now()
	success := false
	var errorMsg string

	// Parse the relay URL for validation
	_, err = url.Parse(decodedURL)
	if err != nil {
		errorMsg = "invalid URL format"
		log.ClientAPI().Debug("Failed to parse relay URL",
			"relay_url", decodedURL,
			"error", err)
	} else {
		// Create WebSocket connection using golang.org/x/net/websocket
		origin := "http://localhost/"

		// Create connection with timeout
		done := make(chan error, 1)
		var conn *websocket.Conn

		go func() {
			var dialErr error
			conn, dialErr = websocket.Dial(decodedURL, "", origin)
			done <- dialErr
		}()

		// Wait for connection with timeout
		select {
		case err := <-done:
			if err != nil {
				errorMsg = err.Error()
				log.ClientAPI().Debug("Relay ping failed",
					"relay_url", decodedURL,
					"error", err,
					"duration_ms", time.Since(startTime).Milliseconds())
			} else {
				success = true
				if conn != nil {
					conn.Close() // Close immediately after successful connection
				}
				log.ClientAPI().Debug("Relay ping successful",
					"relay_url", decodedURL,
					"duration_ms", time.Since(startTime).Milliseconds())
			}
		case <-time.After(5 * time.Second):
			errorMsg = "connection timeout"
			log.ClientAPI().Debug("Relay ping timeout",
				"relay_url", decodedURL,
				"timeout_seconds", 5,
				"duration_ms", time.Since(startTime).Milliseconds())
		}
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Prepare response
	response := map[string]interface{}{
		"success":       success,
		"response_time": responseTime,
		"relay":         decodedURL,
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
			"relay_url", decodedURL,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Relay ping completed",
		"relay_url", decodedURL,
		"success", success,
		"response_time_ms", responseTime,
		"client_ip", r.RemoteAddr)
}
