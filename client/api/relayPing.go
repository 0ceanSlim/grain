package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// PingHandler pings any relay and returns response time and connection status
// Now supports domain-in-path with auto ws/wss detection
// Usage: GET /api/v1/ping/relay.damus.io
func PingHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		log.ClientAPI().Warn("Invalid HTTP method for ping",
			"method", r.Method,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract relay domain from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ping/")
	relayDomain := strings.TrimSpace(path)

	if relayDomain == "" {
		log.ClientAPI().Warn("Missing relay domain parameter",
			"client_ip", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Relay domain is required in URL path"})
		return
	}

	log.ClientAPI().Debug("Starting relay ping with protocol detection",
		"domain", relayDomain,
		"client_ip", r.RemoteAddr)

	// Clean domain (remove any protocol prefix if user included it)
	cleanDomain := strings.TrimPrefix(relayDomain, "wss://")
	cleanDomain = strings.TrimPrefix(cleanDomain, "ws://")
	cleanDomain = strings.TrimSuffix(cleanDomain, "/")

	// Try both protocols and return the successful one
	startTime := time.Now()
	success := false
	var workingURL string
	var errorMsg string

	// Try wss:// first (more secure)
	wssURL := "wss://" + cleanDomain + "/"
	if latency := pingSpecificURL(wssURL); latency >= 0 {
		success = true
		workingURL = wssURL
		log.ClientAPI().Debug("Relay ping successful via wss",
			"domain", cleanDomain,
			"url", wssURL,
			"latency_ms", latency)
	} else {
		// Try ws:// as fallback
		wsURL := "ws://" + cleanDomain + "/"
		if latency := pingSpecificURL(wsURL); latency >= 0 {
			success = true
			workingURL = wsURL
			log.ClientAPI().Debug("Relay ping successful via ws",
				"domain", cleanDomain,
				"url", wsURL,
				"latency_ms", latency)
		} else {
			errorMsg = "Unable to connect via ws:// or wss://"
			log.ClientAPI().Debug("Relay ping failed on both protocols",
				"domain", cleanDomain,
				"wss_url", wssURL,
				"ws_url", wsURL)
		}
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Prepare response in the same format as before
	response := map[string]interface{}{
		"success":       success,
		"response_time": responseTime,
		"relay":         workingURL, // Returns the working URL with protocol
	}

	if !success {
		response["error"] = errorMsg
		response["relay"] = cleanDomain // Return just domain if failed
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Return response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode ping response",
			"error", err,
			"domain", cleanDomain,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Relay ping completed",
		"domain", cleanDomain,
		"success", success,
		"working_url", workingURL,
		"response_time_ms", responseTime,
		"client_ip", r.RemoteAddr)
}

// pingSpecificURL pings a specific URL and returns latency in milliseconds, or -1 if failed
func pingSpecificURL(url string) int64 {
	startTime := time.Now()

	// Create WebSocket connection with timeout
	done := make(chan error, 1)
	var conn *websocket.Conn

	go func() {
		origin := "http://localhost/"
		var err error
		conn, err = websocket.Dial(url, "", origin)
		done <- err
	}()

	// Wait for connection with timeout
	select {
	case err := <-done:
		if err != nil {
			return -1
		}

		// Successfully connected
		latency := time.Since(startTime).Milliseconds()

		// Close connection immediately
		if conn != nil {
			conn.Close()
		}

		return latency

	case <-time.After(5 * time.Second):
		return -1
	}
}
