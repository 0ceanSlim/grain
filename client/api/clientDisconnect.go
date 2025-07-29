package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ClientDisconnectHandler disconnects the client from a relay
// Usage: POST /api/v1/client/disconnect/relay.damus.io
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

	// Extract relay domain from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/client/disconnect/")
	relayDomain := strings.TrimSpace(path)

	if relayDomain == "" {
		log.ClientAPI().Warn("Missing relay domain parameter",
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Relay domain is required in URL path"})
		return
	}

	publicKey := userSession.PublicKey

	log.ClientAPI().Debug("Disconnecting client from relay",
		"domain", relayDomain,
		"client_ip", r.RemoteAddr,
		"user", publicKey)

	// Find the relay URL in user's cached relays (could be ws:// or wss://)
	cachedRelays, err := cache.GetUserClientRelays(publicKey)
	if err != nil {
		log.ClientAPI().Error("Failed to get cached relays",
			"user", publicKey,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"relay":   relayDomain,
			"error":   "Failed to get user's relay list",
		})
		return
	}

	// Clean domain for matching
	cleanDomain := strings.TrimPrefix(relayDomain, "wss://")
	cleanDomain = strings.TrimPrefix(cleanDomain, "ws://")
	cleanDomain = strings.TrimSuffix(cleanDomain, "/")

	var foundRelayURL string
	for _, relay := range cachedRelays {
		// Check if this relay matches the domain
		if strings.Contains(relay.URL, cleanDomain) {
			foundRelayURL = relay.URL
			break
		}
	}

	if foundRelayURL == "" {
		log.ClientAPI().Warn("Relay not found in user's relay list",
			"domain", relayDomain,
			"user", publicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"relay":   relayDomain,
			"error":   "Relay not found in user's relay list",
		})
		return
	}

	// Disconnect from core client pool
	coreClient := connection.GetCoreClient()
	if coreClient != nil {
		if err := disconnectRelayFromPool(coreClient, foundRelayURL); err != nil {
			log.ClientAPI().Warn("Failed to disconnect from relay pool",
				"relay", foundRelayURL,
				"error", err,
				"user", publicKey)
			// Continue anyway to remove from cache
		} else {
			log.ClientAPI().Info("Successfully disconnected from relay pool",
				"relay", foundRelayURL,
				"user", publicKey)
		}
	}

	// Remove from user's cached client relays
	if err := cache.RemoveClientRelay(publicKey, foundRelayURL); err != nil {
		log.ClientAPI().Error("Failed to remove relay from cache",
			"relay", foundRelayURL,
			"error", err,
			"user", publicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"relay":   foundRelayURL,
			"error":   "Failed to remove relay from user's relay list",
		})
		return
	}

	log.ClientAPI().Info("Successfully disconnected and removed relay",
		"user", publicKey,
		"relay", foundRelayURL,
		"domain", relayDomain)

	// Return success response
	response := map[string]interface{}{
		"success":   true,
		"relay":     foundRelayURL,
		"message":   "Successfully disconnected from relay",
		"connected": false,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode disconnect response",
			"error", err,
			"relay_url", foundRelayURL,
			"client_ip", r.RemoteAddr,
			"user", publicKey)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Client disconnected from relay",
		"relay_url", foundRelayURL,
		"client_ip", r.RemoteAddr,
		"user", publicKey)
}

// disconnectRelayFromPool disconnects a relay from the core client pool
func disconnectRelayFromPool(coreClient interface{}, relayURL string) error {
	// Cast the interface to the actual core client type
	client, ok := coreClient.(*core.Client)
	if !ok {
		return fmt.Errorf("invalid core client type")
	}

	log.ClientAPI().Info("Disconnecting relay from pool", "relay", relayURL)

	// Use the core client's DisconnectFromRelay method
	if err := client.DisconnectFromRelay(relayURL); err != nil {
		log.ClientAPI().Error("Failed to disconnect relay from pool", "relay", relayURL, "error", err)
		return err
	}

	log.ClientAPI().Info("Successfully disconnected relay from pool", "relay", relayURL)
	return nil
}
