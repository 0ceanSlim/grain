package connection

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/stream"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// The core client. A config reload closes it and creates another while
// requests and background loops are reading it, so it is swapped atomically,
// and readers take one snapshot (GetCoreClient) and use only that.
var coreClient atomic.Pointer[core.Client]

// Index relays — seed/discovery set used to resolve NIP-65 mailbox lists
// and profile metadata for arbitrary users. Per-user relay sets (a user's
// own outbox/inbox/DM relays) live in the per-session cache, separate from
// this app-level set.
var indexRelays atomic.Pointer[[]string]

// startMu guards stopStart, which cancels the background start (connect,
// seed, discovery) of the current core client when it's closed.
var (
	startMu   sync.Mutex
	stopStart context.CancelFunc
)

// InitializeCoreClient sets up the global core client with server configuration
func InitializeCoreClient(serverCfg *cfgType.ServerConfig) error {
	// Create client config from server config (uses defaults if not specified)
	config := core.ConfigFromServerConfig(serverCfg)

	// Validate the configuration
	if err := config.Validate(); err != nil {
		log.ClientConnection().Error("Invalid client configuration", "error", err)
		return err
	}

	cc := core.NewClient(config)

	// Pin index/seed relays so the idle sweeper never evicts them — they must
	// stay connected to resolve relay lists and metadata for arbitrary users.
	cc.PinRelays(config.IndexRelays...)

	ctx, cancel := context.WithCancel(context.Background())
	startMu.Lock()
	if stopStart != nil {
		stopStart()
	}
	stopStart = cancel
	startMu.Unlock()

	SetIndexRelays(config.IndexRelays)
	coreClient.Store(cc)

	// Live-sync (#87): start a user's own-event subscription when their first
	// page opens the SSE channel, and stop it when the last one disconnects.
	stream.Default().SetLifecycle(StartLiveSync, StopLiveSync)

	// Connect to index relays asynchronously. Relay startup must never
	// block on outbound network — when index relays are unreachable, the
	// retry loop can take 30+ seconds and leave the HTTP server unable
	// to accept connections. Subsystems that need a connection (mutelist
	// fetch, dashboard profile fetch) tolerate the empty pool: they fall
	// back gracefully or simply return empty results until connections
	// establish in the background.
	//
	// Everything here runs on cc, the client this call made, never on the
	// package's current one: a config reload can close cc and install
	// another while the connect is still in flight.
	go func() {
		if err := cc.ConnectToRelaysWithRetry(config.IndexRelays, config.RetryAttempts); err != nil {
			log.ClientConnection().Warn("Failed to connect to index relays during initialization - relay will operate in offline mode",
				"error", err,
				"relay_count", len(config.IndexRelays))
			return
		}
		if ctx.Err() != nil {
			return // closed by a reload while connecting
		}
		log.ClientConnection().Info("Core client connected to index relays",
			"relay_count", len(config.IndexRelays))

		// Seed the routing directory broadly from the indexers, and bootstrap
		// NIP-66 relay discovery for the browser (#104) — in independent
		// goroutines so a slow seed fetch can't delay discovery or vice versa
		// (both do bulk, timeout-bounded network I/O). Discovery finds monitors
		// (kind 10166) reachable from the relays grain touches, then pulls their
		// relay records (kind 30166) that source the known-relays browser. Reach
		// widens as general relays join the pool; periodic re-discovery +
		// staleness eviction land in Phase 3.
		go cc.SeedKnownRelays()
		go cc.DiscoverRelays(ctx)
	}()

	log.ClientConnection().Info("Core client initialized successfully",
		"offline_capable", true,
		"connection_timeout", config.ConnectionTimeout,
		"retry_attempts", config.RetryAttempts)
	return nil
}

// GetCoreClient returns the core client instance, or nil before
// initialization and after close. Callers keep the pointer they got for the
// whole operation rather than asking again.
func GetCoreClient() *core.Client {
	return coreClient.Load()
}

// CloseCoreClient closes the core client connections
func CloseCoreClient() error {
	startMu.Lock()
	if stopStart != nil {
		stopStart()
		stopStart = nil
	}
	startMu.Unlock()

	if cc := coreClient.Swap(nil); cc != nil {
		log.ClientConnection().Info("Closing core client connections")
		releaseAllUsers()
		// Actually tear down the relay pool. Its per-connection read/write
		// goroutines and sockets don't stop just because we drop the
		// pointer — Close() shuts down all subscriptions and relay
		// connections, so a config-reload restart doesn't leak the old
		// pool (#93).
		if err := cc.Close(); err != nil {
			log.ClientConnection().Warn("Error closing core client relay pool", "error", err)
		}
	}
	return nil
}

// SetIndexRelays sets the index/seed relays used for discovery
func SetIndexRelays(relays []string) {
	indexRelays.Store(&relays)
	log.ClientConnection().Debug("Index relays set", "relay_count", len(relays))
}

// GetIndexRelays returns the configured index/seed relays
func GetIndexRelays() []string {
	if r := indexRelays.Load(); r != nil {
		return *r
	}
	return nil
}

// IsCoreClientInitialized checks if the core client is properly initialized
func IsCoreClientInitialized() bool {
	return coreClient.Load() != nil
}
