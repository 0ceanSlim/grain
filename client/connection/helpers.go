package connection

import (
	"fmt"
	"time"

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

	if err := coreClient.ConnectToRelaysWithRetry(indexRelays, 3); err != nil {
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
		"index_relays":     indexRelays,
	}
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
			expectedCount := len(indexRelays)
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
