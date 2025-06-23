package connection

import (
	"fmt"

	"github.com/0ceanslim/grain/server/utils/log"
)

// EnsureRelayConnections checks and reconnects to relays if needed
func EnsureRelayConnections() error {
	if coreClient == nil {
		return fmt.Errorf("core client not initialized")
	}
	
	// Check current connections
	connectedRelays := coreClient.GetConnectedRelays()
	log.Util().Debug("Current relay connections", "connected_count", len(connectedRelays))
	
	// If we have some connections, we're good
	if len(connectedRelays) > 0 {
		return nil
	}
	
	// No connections, try to reconnect
	log.Util().Warn("No relay connections found, attempting to reconnect")
	
	if err := coreClient.ConnectToRelaysWithRetry(appRelays, 3); err != nil {
		log.Util().Error("Failed to reconnect to relays", "error", err)
		return err
	}
	
	// Verify we now have connections
	connectedRelays = coreClient.GetConnectedRelays()
	if len(connectedRelays) == 0 {
		return fmt.Errorf("still no relay connections after reconnection attempt")
	}
	
	log.Util().Info("Successfully reconnected to relays", "connected_count", len(connectedRelays))
	return nil
}

// GetCoreClientStatus returns status information about the core client
func GetCoreClientStatus() map[string]interface{} {
	if coreClient == nil {
		return map[string]interface{}{
			"initialized": false,
			"error": "core client not initialized",
		}
	}
	
	connectedRelays := coreClient.GetConnectedRelays()
	
	return map[string]interface{}{
		"initialized": true,
		"connected_relays": connectedRelays,
		"connected_count": len(connectedRelays),
		"app_relays": appRelays,
	}
}

// ReinitializeCoreClient reinitializes the core client (for recovery)
func ReinitializeCoreClient() error {
	log.Util().Warn("Reinitializing core client")
	
	// Close existing client if any
	if coreClient != nil {
		coreClient.Close()
	}
	
	// Reinitialize
	return InitializeCoreClient(appRelays)
}