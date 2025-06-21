package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ConnectRelayRequest represents a request to connect to a relay
type ConnectRelayRequest struct {
	RelayURL string `json:"relayUrl"`
}

// ConnectRelayResponse represents the response from connecting to a relay
type ConnectRelayResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// RelayStatusResponse represents the status of relay connections
type RelayStatusResponse struct {
	ConnectedRelays []string `json:"connectedRelays"`
	TotalConnected  int      `json:"totalConnected"`
}

// ConnectRelayHandler handles requests to connect to a new relay
func ConnectRelayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication
	session := auth.EnhancedSessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req ConnectRelayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Util().Error("Failed to parse connect relay request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RelayURL == "" {
		sendRelayResponse(w, ConnectRelayResponse{
			Success: false,
			Error:   "Relay URL is required",
		})
		return
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		sendRelayResponse(w, ConnectRelayResponse{
			Success: false,
			Error:   "Client not available",
		})
		return
	}

	// Connect to relay
	if err := coreClient.ConnectToRelays([]string{req.RelayURL}); err != nil {
		log.Util().Error("Failed to connect to relay", "relay", req.RelayURL, "error", err)
		sendRelayResponse(w, ConnectRelayResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	log.Util().Info("Successfully connected to relay", "relay", req.RelayURL)
	sendRelayResponse(w, ConnectRelayResponse{
		Success: true,
		Message: "Connected to relay successfully",
	})
}

// DisconnectRelayHandler handles requests to disconnect from a relay
func DisconnectRelayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication
	session := auth.EnhancedSessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req ConnectRelayRequest // Reusing the same struct since it has the same fields
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Util().Error("Failed to parse disconnect relay request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RelayURL == "" {
		sendRelayResponse(w, ConnectRelayResponse{
			Success: false,
			Error:   "Relay URL is required",
		})
		return
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		sendRelayResponse(w, ConnectRelayResponse{
			Success: false,
			Error:   "Client not available",
		})
		return
	}

	// Note: We'll need to add a method to disconnect from specific relays in the core client
	// For now, we'll return a placeholder response
	log.Util().Info("Disconnect relay requested", "relay", req.RelayURL)
	sendRelayResponse(w, ConnectRelayResponse{
		Success: true,
		Message: "Relay disconnect requested (feature in development)",
	})
}

// GetRelayStatusHandler returns the status of all relay connections
func GetRelayStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	session := auth.EnhancedSessionMgr.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get core client
	coreClient := auth.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Get connected relays (we'll need to access the relay pool)
	// For now, return a placeholder response
	response := RelayStatusResponse{
		ConnectedRelays: []string{}, // Would get from coreClient.relayPool.GetConnectedRelays()
		TotalConnected:  0,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode relay status response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// sendRelayResponse sends a JSON response for relay operations
func sendRelayResponse(w http.ResponseWriter, response ConnectRelayResponse) {
	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Util().Error("Failed to encode relay response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}