package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/server/utils/log"
)

// RelayStatus represents the status of a relay connection
type RelayStatus struct {
	URL       string `json:"url"`
	Connected bool   `json:"connected"`
	Status    string `json:"status"`
}

// ClientRelaysResponse represents the response for client relays
type ClientRelaysResponse struct {
	Relays []RelayStatus `json:"relays"`
	Count  int           `json:"count"`
}

// ClientRelaysHandler returns the client's configured relays and their status
func ClientRelaysHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		log.ClientAPI().Warn("Invalid HTTP method for client relays",
			"method", r.Method,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.ClientAPI().Debug("Getting client relay status",
		"client_ip", r.RemoteAddr)

	// TODO: Get actual relay list from client configuration
	// This should come from your client's relay manager or configuration
	// For now, using example relays
	relays := []RelayStatus{
		{
			URL:       "wss://relay.damus.io",
			Connected: true,
			Status:    "connected",
		},
		{
			URL:       "wss://nos.lol",
			Connected: true,
			Status:    "connected",
		},
		{
			URL:       "wss://relay.nostr.band",
			Connected: false,
			Status:    "disconnected",
		},
	}

	// Prepare response
	response := ClientRelaysResponse{
		Relays: relays,
		Count:  len(relays),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode relays response",
			"error", err,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Client relay status retrieved",
		"relay_count", len(relays),
		"client_ip", r.RemoteAddr)
}
