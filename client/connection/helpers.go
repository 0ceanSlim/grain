package connection

import (
	"context"
	"fmt"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// EnsureRelayConnections is the demand-side guard used by API/data callers
// that need at least one usable relay connection before issuing a query.
// It returns nil as soon as any relay is connected; if none are it forces
// a reconnect attempt against the full index-relay list. This is *not* the
// right primitive for filling a partial pool — see TopUpRelayConnections
// for that, and #68 for the production bug where confusing the two
// semantics let the upstream pool sit at 1/5 indefinitely.
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

// TopUpRelayConnections attempts to bring the index-relay pool up to its
// configured target by dialing every relay that isn't currently
// connected. Already-connected relays are left alone (the pool's Connect
// is a no-op for them). Returns the (before, after) connection counts so
// the caller can log meaningfully — this exists because the health check
// previously logged "reconnection successful" whenever EnsureRelayConnections
// returned nil, which it does on any-connection-exists, regardless of
// whether the deficit was actually closed (#68).
func TopUpRelayConnections() (before, after int, err error) {
	if coreClient == nil {
		return 0, 0, fmt.Errorf("core client not initialized")
	}

	beforeRelays := coreClient.GetConnectedRelays()
	before = len(beforeRelays)

	connected := make(map[string]struct{}, before)
	for _, url := range beforeRelays {
		connected[url] = struct{}{}
	}

	// Dial every configured relay that isn't currently connected.
	missing := make([]string, 0, len(indexRelays))
	for _, url := range indexRelays {
		if _, ok := connected[url]; !ok {
			missing = append(missing, url)
		}
	}

	if len(missing) == 0 {
		return before, before, nil
	}

	// ConnectToRelays attempts each URL once; failures are logged inside
	// the core client. We don't propagate per-relay failures here — the
	// caller cares about the resulting count, not the individual errors.
	dialErr := coreClient.ConnectToRelays(missing)

	after = len(coreClient.GetConnectedRelays())
	if after == before && dialErr != nil {
		return before, after, dialErr
	}
	return before, after, nil
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
	stats := coreClient.PoolStats()

	return map[string]interface{}{
		"initialized":      true,
		"connected_relays": connectedRelays,
		"connected_count":  len(connectedRelays),
		"index_relays":     indexRelays,
		"pool_known":       stats.Known,     // relays the client is aware of
		"pool_total":       stats.Total,     // relays tracked in the pool
		"pool_connected":   stats.Connected, // currently connected
		"pool_pinned":      stats.Pinned,    // index/seed relays
		"pool_leased":      stats.Leased,    // in active use
	}
}

// StartRelayEvictionSweeper starts the core client's idle-connection sweeper,
// bounded to ctx so it doesn't outlive the server instance (#93). Companion to
// StartRelayHealthCheck: the health check keeps the pinned index relays up,
// while the sweeper reclaims the per-user connections the outbox pool dials on
// demand once they go idle.
func StartRelayEvictionSweeper(ctx context.Context, interval time.Duration) {
	if coreClient == nil {
		log.ClientConnection().Warn("Core client not initialized; eviction sweeper not started")
		return
	}
	coreClient.StartEvictionSweeper(ctx, interval)
}

// StartRelayDiscoveryRoll starts the core client's periodic NIP-66 discovery
// roll (#104 Phase 3): re-discover relay monitors, refresh their published relay
// sets, and evict monitors whose data has gone stale — keeping the known-relays
// browser a live, self-healing set. Bounded to ctx like the other background
// services. The initial bootstrap pass is kicked at connect time.
func StartRelayDiscoveryRoll(ctx context.Context, interval time.Duration) {
	if coreClient == nil {
		log.ClientConnection().Warn("Core client not initialized; discovery roll not started")
		return
	}
	coreClient.StartDiscoveryRoll(ctx, interval)
}

// StartRelayHealthCheck starts a background goroutine to maintain relay
// connections. It exits when ctx is cancelled so it doesn't outlive the
// server instance that started it (#93).
func StartRelayHealthCheck(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.ClientConnection().Info("Relay health check started", "interval", interval)

		for {
			select {
			case <-ctx.Done():
				log.ClientConnection().Info("Relay health check stopping")
				return
			case <-ticker.C:
			}
			if coreClient == nil {
				log.ClientConnection().Debug("Core client not initialized, skipping health check")
				continue
			}

			expectedCount := len(indexRelays)
			connectedCount := len(coreClient.GetConnectedRelays())

			log.ClientConnection().Debug("Relay health check",
				"connected", connectedCount,
				"expected", expectedCount)

			if connectedCount >= expectedCount {
				continue
			}

			log.ClientConnection().Warn("Relay connection deficit detected, attempting top-up",
				"connected", connectedCount,
				"expected", expectedCount)

			before, after, err := TopUpRelayConnections()
			gained := after - before
			switch {
			case err != nil && gained == 0:
				log.ClientConnection().Error("Health check top-up failed",
					"connected", after,
					"expected", expectedCount,
					"error", err)
			case after >= expectedCount:
				log.ClientConnection().Info("Health check restored full pool",
					"connected", after,
					"expected", expectedCount,
					"gained", gained)
			case gained > 0:
				// Partial recovery — still under target, but progress.
				log.ClientConnection().Info("Health check top-up partial",
					"connected", after,
					"expected", expectedCount,
					"gained", gained)
			default:
				// No progress and no error means every missing relay
				// refused/timed-out the dial. Don't claim success.
				log.ClientConnection().Warn("Health check top-up made no progress",
					"connected", after,
					"expected", expectedCount)
			}
		}
	}()
}
