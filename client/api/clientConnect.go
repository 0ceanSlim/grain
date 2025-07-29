package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ClientConnectHandler connects the client to a relay
// Usage: POST /api/v1/client/connect/relay.damus.io?read=true&write=false
func ClientConnectHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		log.ClientAPI().Warn("Invalid HTTP method for client connect",
			"method", r.Method,
			"client_ip", r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if user is logged in
	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil || userSession.PublicKey == "" {
		log.ClientAPI().Warn("Unauthorized client connect attempt",
			"client_ip", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "You need to login to the client to change the default relays",
		})
		return
	}

	// Extract relay domain from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/client/connect/")
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

	// Parse permission parameters from query string
	readParam := r.URL.Query().Get("read")
	writeParam := r.URL.Query().Get("write")

	// Default permissions (both read and write if no params specified)
	readPermission := true
	writePermission := true

	// Parse read parameter
	if readParam != "" {
		readPermission = strings.ToLower(readParam) == "true"
	}

	// Parse write parameter
	if writeParam != "" {
		writePermission = strings.ToLower(writeParam) == "true"
	}

	// Validate that at least one permission is enabled
	if !readPermission && !writePermission {
		log.ClientAPI().Warn("Invalid permissions - at least one of read or write must be true",
			"domain", relayDomain,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"relay":   relayDomain,
			"error":   "At least one of read or write permissions must be true",
		})
		return
	}

	log.ClientAPI().Debug("Connecting client to relay with protocol detection",
		"domain", relayDomain,
		"read", readPermission,
		"write", writePermission,
		"client_ip", r.RemoteAddr,
		"user", userSession.PublicKey)

	// Try to connect with protocol detection
	workingURL, err := connectWithProtocolDetection(relayDomain)
	if err != nil {
		log.ClientAPI().Error("Failed to connect to relay",
			"domain", relayDomain,
			"error", err,
			"user", userSession.PublicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"relay":   relayDomain,
			"error":   err.Error(),
		})
		return
	}

	// Add to user's cached client relays with specified permissions
	publicKey := userSession.PublicKey
	if err := cache.AddClientRelayWithPermissions(publicKey, workingURL, readPermission, writePermission); err != nil {
		log.ClientAPI().Error("Failed to add relay to cache",
			"relay", workingURL,
			"error", err,
			"user", publicKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"relay":   workingURL,
			"error":   "Failed to add relay to user's relay list",
		})
		return
	}

	log.ClientAPI().Info("Successfully connected and cached relay",
		"user", publicKey,
		"relay", workingURL,
		"domain", relayDomain,
		"read", readPermission,
		"write", writePermission)

	// Return success response with permission info
	response := map[string]interface{}{
		"success":   true,
		"relay":     workingURL,
		"message":   "Successfully connected to relay",
		"connected": true,
		"read":      readPermission,
		"write":     writePermission,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode connect response",
			"error", err,
			"relay_url", workingURL,
			"client_ip", r.RemoteAddr,
			"user", userSession.PublicKey)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.ClientAPI().Info("Client connected to relay",
		"relay_url", workingURL,
		"read", readPermission,
		"write", writePermission,
		"client_ip", r.RemoteAddr,
		"user", userSession.PublicKey)
}

// connectWithProtocolDetection tries both ws and wss, returns working URL
func connectWithProtocolDetection(domain string) (string, error) {
	// Clean domain (remove any protocol prefix if provided)
	cleanDomain := strings.TrimPrefix(domain, "wss://")
	cleanDomain = strings.TrimPrefix(cleanDomain, "ws://")
	cleanDomain = strings.TrimSuffix(cleanDomain, "/")

	// Get core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		return "", fmt.Errorf("core client not available")
	}

	// Try wss:// first (more secure)
	wssURL := "wss://" + cleanDomain + "/"
	log.ClientAPI().Debug("Trying wss connection", "url", wssURL)

	if err := coreClient.ConnectToRelays([]string{wssURL}); err == nil {
		// Check if actually connected
		connectedRelays := coreClient.GetConnectedRelays()
		for _, relay := range connectedRelays {
			if relay == wssURL {
				log.ClientAPI().Info("Successfully connected via wss", "url", wssURL)
				return wssURL, nil
			}
		}
	}

	// Try ws:// as fallback
	wsURL := "ws://" + cleanDomain + "/"
	log.ClientAPI().Debug("Trying ws connection", "url", wsURL)

	if err := coreClient.ConnectToRelays([]string{wsURL}); err == nil {
		// Check if actually connected
		connectedRelays := coreClient.GetConnectedRelays()
		for _, relay := range connectedRelays {
			if relay == wsURL {
				log.ClientAPI().Info("Successfully connected via ws", "url", wsURL)
				return wsURL, nil
			}
		}
	}

	return "", fmt.Errorf("unable to connect via ws:// or wss://")
}
