package connection

import (
	"fmt"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils/log"
)

// EnsureRelayConnections checks and reconnects to relays if needed
func EnsureRelayConnections() error {
	if coreClient == nil {
		return fmt.Errorf("core client not initialized")
	}

	// Check current connections
	connectedRelays := coreClient.GetConnectedRelays()
	log.ClientConnection().Debug("Current relay connections", "connected_count", len(connectedRelays))

	// If we have some connections, we're good
	if len(connectedRelays) > 0 {
		return nil
	}

	// No connections, try to reconnect
	log.ClientConnection().Warn("No relay connections found, attempting to reconnect")

	if err := coreClient.ConnectToRelaysWithRetry(clientRelays, 3); err != nil {
		log.ClientConnection().Error("Failed to reconnect to relays", "error", err)
		return err
	}

	// Verify we now have connections
	connectedRelays = coreClient.GetConnectedRelays()
	if len(connectedRelays) == 0 {
		return fmt.Errorf("still no relay connections after reconnection attempt")
	}

	log.ClientConnection().Info("Successfully reconnected to relays", "connected_count", len(connectedRelays))
	return nil
}

// GetCoreClientStatus returns status information about the core client
func GetCoreClientStatus() map[string]interface{} {
	if coreClient == nil {
		return map[string]interface{}{
			"initialized": false,
			"error":       "core client not initialized",
		}
	}

	connectedRelays := coreClient.GetConnectedRelays()

	return map[string]interface{}{
		"initialized":      true,
		"connected_relays": connectedRelays,
		"connected_count":  len(connectedRelays),
		"client_relays":    clientRelays,
	}
}

// ReinitializeCoreClient reinitializes the core client (for recovery)
func ReinitializeCoreClient() error {
	log.ClientConnection().Warn("Reinitializing core client")

	// Close existing client if any
	if coreClient != nil {
		coreClient.Close()
	}

	// Get current server configuration for reinitialization
	serverCfg := config.GetConfig()
	if serverCfg == nil {
		// Fallback to last known config if current config unavailable
		serverCfg = lastServerConfig
	}

	// Reinitialize with current configuration
	return InitializeCoreClient(serverCfg)
}

// StartRelayHealthCheck starts a background goroutine to maintain relay connections
func StartRelayHealthCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.ClientConnection().Info("Relay health check started", "interval", interval)

		for range ticker.C {
			if coreClient == nil {
				log.ClientConnection().Debug("Core client not initialized, skipping health check")
				continue
			}

			// Check current connections
			connectedRelays := coreClient.GetConnectedRelays()
			expectedCount := len(clientRelays)
			connectedCount := len(connectedRelays)

			log.ClientConnection().Debug("Relay health check",
				"connected", connectedCount,
				"expected", expectedCount)

			// If we have fewer connections than expected, try to reconnect
			if connectedCount < expectedCount {
				log.ClientConnection().Warn("Relay connection deficit detected, attempting reconnection",
					"connected", connectedCount,
					"expected", expectedCount)

				if err := EnsureRelayConnections(); err != nil {
					log.ClientConnection().Error("Health check reconnection failed", "error", err)
				} else {
					log.ClientConnection().Info("Health check reconnection successful",
						"connected", len(coreClient.GetConnectedRelays()))
				}
			}
		}
	}()
}
