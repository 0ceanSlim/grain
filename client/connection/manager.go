package connection

import (
	"github.com/0ceanslim/grain/client/core"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Global core client instance
var coreClient *core.Client

// Application relays for initial discovery
var clientRelays []string

// Store reference to server config for reinitialization
var lastServerConfig *cfgType.ServerConfig

// InitializeCoreClient sets up the global core client with server configuration
func InitializeCoreClient(serverCfg *cfgType.ServerConfig) error {
	// Store config for potential reinitialization
	lastServerConfig = serverCfg

	// Create client config from server config (uses defaults if not specified)
	config := core.ConfigFromServerConfig(serverCfg)

	// Validate the configuration
	if err := config.Validate(); err != nil {
		log.ClientConnection().Error("Invalid client configuration", "error", err)
		return err
	}

	coreClient = core.NewClient(config)

	// Store relays for later use
	clientRelays = config.DefaultRelays

	// Connect to default relays asynchronously. Relay startup must never
	// block on outbound network — when defaults are unreachable, the
	// retry loop can take 30+ seconds and leave the HTTP server unable
	// to accept connections. Subsystems that need a connection (mutelist
	// fetch, dashboard profile fetch) tolerate the empty pool: they fall
	// back gracefully or simply return empty results until connections
	// establish in the background.
	go func() {
		if err := coreClient.ConnectToRelaysWithRetry(config.DefaultRelays, config.RetryAttempts); err != nil {
			log.ClientConnection().Warn("Failed to connect to relays during initialization - relay will operate in offline mode",
				"error", err,
				"relay_count", len(config.DefaultRelays))
		} else {
			log.ClientConnection().Info("Core client connected to default relays",
				"relay_count", len(config.DefaultRelays))
		}
	}()

	log.ClientConnection().Info("Core client initialized successfully",
		"offline_capable", true,
		"connection_timeout", config.ConnectionTimeout,
		"retry_attempts", config.RetryAttempts)
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

// SetClientRelays sets the application relays for initial discovery
func SetClientRelays(relays []string) {
	clientRelays = relays
	log.ClientConnection().Debug("App relays set", "relay_count", len(relays))
}

// GetClientRelays returns the configured application relays
func GetClientRelays() []string {
	return clientRelays
}

// IsCoreClientInitialized checks if the core client is properly initialized
func IsCoreClientInitialized() bool {
	return coreClient != nil
}
