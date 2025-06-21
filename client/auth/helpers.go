// client/auth/helpers.go
package auth

import (
	"fmt"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/server/utils/log"
)

// InitializeCoreClient sets up the global core client with retry
func InitializeCoreClient(relays []string) error {
	config := core.DefaultConfig()
	config.DefaultRelays = relays
	
	coreClient = core.NewClient(config)
	
	// Connect to default relays with retry
	if err := coreClient.ConnectToRelaysWithRetry(relays, 3); err != nil {
		log.Util().Error("Failed to connect to relays after retries", "error", err)
		return err
	}
	
	log.Util().Info("Core client initialized", "relay_count", len(relays))
	return nil
}

// SetAppRelays sets the application relays for initial discovery
func SetAppRelays(relays []string) {
	appRelays = relays
	log.Util().Debug("App relays set", "relay_count", len(relays))
}

// GetCoreClient returns the core client instance
func GetCoreClient() *core.Client {
	return coreClient
}

// CloseCoreClient closes the core client connections
func CloseCoreClient() error {
	if coreClient != nil {
		// Assuming core client has a Close method
		// Adjust based on your actual core.Client implementation
		log.Util().Info("Closing core client connections")
		coreClient = nil
	}
	return nil
}

// GetAppRelays returns the configured application relays
func GetAppRelays() []string {
	return appRelays
}

// IsSessionManagerInitialized checks if the session manager is properly initialized
func IsSessionManagerInitialized() bool {
	return EnhancedSessionMgr != nil
}

// IsCoreClientInitialized checks if the core client is properly initialized
func IsCoreClientInitialized() bool {
	return coreClient != nil
}

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

// FetchAndCacheUserDataWithCoreClient fetches user data using the core client
func FetchAndCacheUserDataWithCoreClient(publicKey string) error {
	log.Util().Debug("Fetching fresh user data with core client", "pubkey", publicKey)

	// Ensure we have connected relays before proceeding
	if err := EnsureRelayConnections(); err != nil {
		return fmt.Errorf("failed to ensure relay connections: %w", err)
	}

	// First, try to get user's mailboxes using connected relays
	var relaysForMetadata []string
	mailboxes, err := coreClient.GetUserRelays(publicKey)
	if err != nil {
		log.Util().Warn("Failed to fetch mailboxes, using app relays", "pubkey", publicKey, "error", err)
		relaysForMetadata = appRelays
	} else if mailboxes != nil {
		// Get user's preferred relays
		userRelays := mailboxes.ToStringSlice()
		log.Util().Debug("User has preferred relays", "pubkey", publicKey, "relay_count", len(userRelays))
		
		// BUT: Use connected app relays for profile fetch to ensure success
		// This is more reliable than trying to connect to user's personal relays
		connectedRelays := coreClient.GetConnectedRelays()
		if len(connectedRelays) > 0 {
			relaysForMetadata = connectedRelays
			log.Util().Debug("Using connected app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
		} else {
			relaysForMetadata = appRelays
		}
	}

	// Use app relays as fallback
	if len(relaysForMetadata) == 0 {
		relaysForMetadata = appRelays
		log.Util().Info("Using app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
	}

	// Fetch user metadata (profile) using connected relays
	userMetadata, err := coreClient.GetUserProfile(publicKey, relaysForMetadata)
	if err != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", err)
	}

	// Cache the data using the cache package function
	cache.CacheUserDataFromObjects(publicKey, userMetadata, mailboxes)

	log.Util().Info("User data fetched and cached successfully", "pubkey", publicKey)
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