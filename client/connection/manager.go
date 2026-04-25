package connection

import (
	"github.com/0ceanslim/grain/client/core"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Global core client instance
var coreClient *core.Client

// Index relays — seed/discovery set used to resolve NIP-65 mailbox lists
// and profile metadata for arbitrary users. Per-user relay sets (a user's
// own outbox/inbox/DM relays) live in the per-session cache, separate from
// this app-level set.
var indexRelays []string

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
	indexRelays = config.IndexRelays

	// Connect to index relays asynchronously. Relay startup must never
	// block on outbound network — when index relays are unreachable, the
	// retry loop can take 30+ seconds and leave the HTTP server unable
	// to accept connections. Subsystems that need a connection (mutelist
	// fetch, dashboard profile fetch) tolerate the empty pool: they fall
	// back gracefully or simply return empty results until connections
	// establish in the background.
	go func() {
		if err := coreClient.ConnectToRelaysWithRetry(config.IndexRelays, config.RetryAttempts); err != nil {
			log.ClientConnection().Warn("Failed to connect to index relays during initialization - relay will operate in offline mode",
				"error", err,
				"relay_count", len(config.IndexRelays))
		} else {
			log.ClientConnection().Info("Core client connected to index relays",
				"relay_count", len(config.IndexRelays))
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

// SetIndexRelays sets the index/seed relays used for discovery
func SetIndexRelays(relays []string) {
	indexRelays = relays
	log.ClientConnection().Debug("Index relays set", "relay_count", len(relays))
}

// GetIndexRelays returns the configured index/seed relays
func GetIndexRelays() []string {
	return indexRelays
}

// IsCoreClientInitialized checks if the core client is properly initialized
func IsCoreClientInitialized() bool {
	return coreClient != nil
}
