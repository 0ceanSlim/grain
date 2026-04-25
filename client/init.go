package client

import (
	"time"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// InitializeClient sets up the client package with server configuration
func InitializeClient(serverCfg *cfgType.ServerConfig) error {
	log.ClientMain().Info("Initializing client package with configurable settings")

	// Initialize session manager
	if err := initializeSessionManager(); err != nil {
		return err
	}

	// Initialize core client with server configuration
	if err := connection.InitializeCoreClient(serverCfg); err != nil {
		return err
	}

	// Set index relays for discovery (from config or built-in indexer seed list).
	// The fallback set mirrors the indexer-relay role from #56: relays that
	// host metadata and relay lists for everyone, used to resolve NIP-65 /
	// DM-relay lists for arbitrary users.
	if serverCfg != nil && len(serverCfg.Client.IndexRelays) > 0 {
		connection.SetIndexRelays(serverCfg.Client.IndexRelays)
	} else {
		indexRelays := []string{
			"wss://profiles.nostr1.com",
			"wss://directory.yabu.me",
			"wss://user.kindpag.es",
			"wss://indexer.coracle.social",
			"wss://purplepag.es",
		}
		connection.SetIndexRelays(indexRelays)
	}

	// Start background session cleanup
	startSessionCleanup()

	// Start cache cleanup
	cache.StartCacheCleanup()

	// Start relay connection health check (check every 5 minutes)
	connection.StartRelayHealthCheck(5 * time.Minute)

	log.ClientMain().Info("Client package initialized successfully")
	return nil
}

// initializeSessionManager sets up the  session manager
func initializeSessionManager() error {
	session.SessionMgr = session.NewSessionManager()
	if session.SessionMgr == nil {
		return &ClientInitError{Message: "failed to create  session manager"}
	}

	log.ClientMain().Debug(" session manager initialized")
	return nil
}

// startSessionCleanup starts a background goroutine to clean up expired sessions
func startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute) // Clean up every 30 minutes
		defer ticker.Stop()

		for range ticker.C {
			if session.SessionMgr != nil {
				// Clean up sessions older than 24 hours of inactivity
				session.SessionMgr.CleanupSessions(24 * time.Hour)

				// Log session statistics
				stats := session.SessionMgr.GetSessionStats()
				log.ClientMain().Debug("Session cleanup completed",
					"total_sessions", stats["total_sessions"],
					"read_only", stats["read_only"],
					"write_mode", stats["write_mode"])
			}
		}
	}()

	log.ClientMain().Debug("Session cleanup routine started")
}

// ShutdownClient gracefully shuts down the client package
func ShutdownClient() error {
	log.ClientMain().Info("Shutting down client package")

	// Close core client connections
	if err := connection.CloseCoreClient(); err != nil {
		log.ClientMain().Error("Error closing core client", "error", err)
		return err
	}

	// Clear session manager
	session.SessionMgr = nil

	log.ClientMain().Info("Client package shutdown complete")
	return nil
}

// GetCoreClient returns the core client instance for advanced usage
func GetCoreClient() interface{} {
	return connection.GetCoreClient()
}

// GetSessionStats returns current session statistics
func GetSessionStats() map[string]interface{} {
	if session.SessionMgr == nil {
		return map[string]interface{}{
			"error": "session manager not initialized",
		}
	}

	return session.SessionMgr.GetSessionStats()
}

// ClientInitError represents initialization errors
type ClientInitError struct {
	Message string
}

func (e *ClientInitError) Error() string {
	return "client init error: " + e.Message
}
