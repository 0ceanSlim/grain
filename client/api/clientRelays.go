package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// RelayStatus represents the status of a relay connection
type RelayStatus struct {
	URL         string    `json:"url"`
	Connected   bool      `json:"connected"`
	Status      string    `json:"status"`
	Latency     *int64    `json:"latency,omitempty"` // Ping latency in milliseconds
	LastChecked time.Time `json:"last_checked"`
	Read        bool      `json:"read"`
	Write       bool      `json:"write"`
	AddedAt     time.Time `json:"added_at"`
}

// ClientRelaysResponse represents the response for client relays
type ClientRelaysResponse struct {
	Relays []RelayStatus `json:"relays"`
	Count  int           `json:"count"`
}

// ClientRelaysHandler returns the client's configured relays and their status
// Works with or without authentication - returns user relays if authenticated, app relays if not
func ClientRelaysHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		log.ClientAPI().Warn("Invalid HTTP method for client relays",
			"method", r.Method,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication - but don't fail if not authenticated
	userSession := session.SessionMgr.GetCurrentUser(r)
	var publicKey string
	var isAuthenticated bool

	if userSession != nil && userSession.PublicKey != "" {
		publicKey = userSession.PublicKey
		isAuthenticated = true
		log.ClientAPI().Debug("Getting authenticated client relay status", "pubkey", publicKey, "client_ip", r.RemoteAddr)
	} else {
		isAuthenticated = false
		log.ClientAPI().Debug("Getting public relay status (no authentication)", "client_ip", r.RemoteAddr)
	}

	var relayStatuses []RelayStatus
	var err error

	if isAuthenticated {
		// Get user's cached client relays
		relayStatuses, err = getUserRelayStatus(publicKey, r)
		if err != nil {
			log.ClientAPI().Error("Failed to get user client relays", "pubkey", publicKey, "error", err)
			http.Error(w, "Failed to retrieve client relays", http.StatusInternalServerError)
			return
		}
	} else {
		// Get default app relays status
		relayStatuses = getDefaultRelayStatus(r)
	}

	// Prepare response
	response := ClientRelaysResponse{
		Relays: relayStatuses,
		Count:  len(relayStatuses),
	}

	log.ClientAPI().Info("Client relay status retrieved",
		"authenticated", isAuthenticated,
		"pubkey", publicKey,
		"relay_count", len(relayStatuses),
		"ping_enabled", r.URL.Query().Get("ping") == "true",
		"client_ip", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode relays response",
			"error", err,
			"pubkey", publicKey,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getUserRelayStatus gets relay status for authenticated user
func getUserRelayStatus(publicKey string, r *http.Request) ([]RelayStatus, error) {
	// Get user's cached client relays
	clientRelays, err := cache.GetUserClientRelays(publicKey)
	if err != nil {
		return nil, err
	}

	if len(clientRelays) == 0 {
		log.ClientAPI().Warn("No client relays found for user", "pubkey", publicKey)
		// Return empty response instead of error
		return []RelayStatus{}, nil
	}

	// Get core client to check actual connection status
	coreClient := connection.GetCoreClient()
	var connectedRelays []string
	if coreClient != nil {
		connectedRelays = coreClient.GetConnectedRelays()
		log.ClientAPI().Debug("Core client connected relays", "count", len(connectedRelays), "relays", connectedRelays)
	} else {
		log.ClientAPI().Warn("Core client not available for connection status check")
	}

	// Check if we should ping relays (optional query parameter)
	shouldPing := r.URL.Query().Get("ping") == "true"

	// Build relay status list
	relayStatuses := make([]RelayStatus, len(clientRelays))
	for i, relay := range clientRelays {
		// Check if relay is connected via core client
		isConnected := false
		for _, connectedURL := range connectedRelays {
			if connectedURL == relay.URL {
				isConnected = true
				break
			}
		}

		// Initialize relay status
		relayStatus := RelayStatus{
			URL:         relay.URL,
			Connected:   isConnected,
			Status:      getRelayStatus(isConnected),
			LastChecked: time.Now(),
			Read:        relay.Read,
			Write:       relay.Write,
			AddedAt:     relay.AddedAt,
		}

		// Optionally ping the relay for latency check
		if shouldPing {
			latency := pingRelay(relay.URL)
			if latency >= 0 {
				relayStatus.Latency = &latency
				// Update status based on ping result
				if latency > 0 && !isConnected {
					relayStatus.Status = "reachable"
				}
			} else if !isConnected {
				relayStatus.Status = "unreachable"
			}
		}

		relayStatuses[i] = relayStatus
	}

	return relayStatuses, nil
}

// getDefaultRelayStatus gets relay status for default app relays (unauthenticated)
func getDefaultRelayStatus(r *http.Request) []RelayStatus {
	// Get default client relays
	clientRelays := connection.GetClientRelays()

	if len(clientRelays) == 0 {
		log.ClientAPI().Warn("No default client relays configured")
		return []RelayStatus{}
	}

	// Get core client to check actual connection status
	coreClient := connection.GetCoreClient()
	var connectedRelays []string
	if coreClient != nil {
		connectedRelays = coreClient.GetConnectedRelays()
		log.ClientAPI().Debug("Core client connected relays for default status", "count", len(connectedRelays), "relays", connectedRelays)
	}

	// Check if we should ping relays (optional query parameter)
	shouldPing := r.URL.Query().Get("ping") == "true"

	// Build relay status list for default relays
	relayStatuses := make([]RelayStatus, len(clientRelays))
	for i, relayURL := range clientRelays {
		// Check if relay is connected via core client
		isConnected := false
		for _, connectedURL := range connectedRelays {
			if connectedURL == relayURL {
				isConnected = true
				break
			}
		}

		// Initialize relay status - default relays have both read and write permissions
		relayStatus := RelayStatus{
			URL:         relayURL,
			Connected:   isConnected,
			Status:      getRelayStatus(isConnected),
			LastChecked: time.Now(),
			Read:        true, // Default app relays have both permissions
			Write:       true,
			AddedAt:     time.Now(), // Default relays don't have a specific added time
		}

		// Optionally ping the relay for latency check
		if shouldPing {
			latency := pingRelay(relayURL)
			if latency >= 0 {
				relayStatus.Latency = &latency
				// Update status based on ping result
				if latency > 0 && !isConnected {
					relayStatus.Status = "reachable"
				}
			} else if !isConnected {
				relayStatus.Status = "unreachable"
			}
		}

		relayStatuses[i] = relayStatus
	}

	return relayStatuses
}

// getRelayStatus returns a human-readable status string
func getRelayStatus(connected bool) string {
	if connected {
		return "connected"
	}
	return "disconnected"
}

// pingRelay attempts to ping a relay and returns latency in milliseconds, or -1 if failed
func pingRelay(relayURL string) int64 {
	// Set timeout for ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()

	// Create WebSocket connection with timeout
	done := make(chan error, 1)
	var conn *websocket.Conn

	go func() {
		origin := "http://localhost/"
		var err error
		conn, err = websocket.Dial(relayURL, "", origin)
		done <- err
	}()

	// Wait for connection with context timeout
	select {
	case err := <-done:
		if err != nil {
			log.ClientAPI().Debug("Relay ping failed", "relay", relayURL, "error", err)
			return -1
		}

		// Successfully connected
		latency := time.Since(startTime).Milliseconds()

		// Close connection immediately
		if conn != nil {
			conn.Close()
		}

		log.ClientAPI().Debug("Relay ping successful", "relay", relayURL, "latency_ms", latency)
		return latency

	case <-ctx.Done():
		log.ClientAPI().Debug("Relay ping timeout", "relay", relayURL)
		return -1
	}
}
