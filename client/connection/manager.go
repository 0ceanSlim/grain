package connection

import (
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Global core client instance
var coreClient *core.Client

// Application relays for initial discovery
var appRelays []string

// InitializeCoreClient sets up the global core client with retry
func InitializeCoreClient(relays []string) error {
	config := core.DefaultConfig()
	config.DefaultRelays = relays
	
	coreClient = core.NewClient(config)
	
	// Connect to default relays with retry
	if err := coreClient.ConnectToRelaysWithRetry(relays, 3); err != nil {
		log.ClientConnection().Error("Failed to connect to relays after retries", "error", err)
		return err
	}
	
	log.ClientConnection().Info("Core client initialized", "relay_count", len(relays))
	return nil
}

// GetCoreClient returns the core client instance
func GetCoreClient() *core.Client {
	return coreClient
}

// CloseCoreClient closes the core client connections
func CloseCoreClient() error {
	if coreClient != nil {
		log.ClientConnection().Info("Closing core client connections")
		coreClient = nil
	}
	return nil
}

// SetAppRelays sets the application relays for initial discovery
func SetAppRelays(relays []string) {
	appRelays = relays
	log.ClientConnection().Debug("App relays set", "relay_count", len(relays))
}

// GetAppRelays returns the configured application relays
func GetAppRelays() []string {
	return appRelays
}

// IsCoreClientInitialized checks if the core client is properly initialized
func IsCoreClientInitialized() bool {
	return coreClient != nil
}