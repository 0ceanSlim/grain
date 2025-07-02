package client

import (
	"time"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// InitializeClient sets up the client package with  session management
func InitializeClient(relays []string) error {
	log.Util().Info("Initializing client package with  session management", "relay_count", len(relays))

	// Initialize  session manager
	if err := initializeSessionManager(); err != nil {
		return err
	}

	// Initialize core client with relays
	if err := connection.InitializeCoreClient(relays); err != nil {
		return err
	}

	// Set app relays for discovery
	connection.SetAppRelays(relays)

	// Start background session cleanup
	startSessionCleanup()

	// Start cache cleanup
	cache.StartCacheCleanup()

	log.Util().Info("Client package initialized successfully with  features")
	return nil
}

// initializeSessionManager sets up the  session manager
func initializeSessionManager() error {
	session.SessionMgr = session.NewSessionManager()
	if session.SessionMgr == nil {
		return &ClientInitError{Message: "failed to create  session manager"}
	}

	log.Util().Debug(" session manager initialized")
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
				log.Util().Debug("Session cleanup completed", 
					"total_sessions", stats["total_sessions"],
					"read_only", stats["read_only"],
					"write_mode", stats["write_mode"])
			}
		}
	}()
	
	log.Util().Debug("Session cleanup routine started")
}

// ShutdownClient gracefully shuts down the client package
func ShutdownClient() error {
	log.Util().Info("Shutting down client package")

	// Close core client connections
	if err := connection.CloseCoreClient(); err != nil {
		log.Util().Error("Error closing core client", "error", err)
		return err
	}

	// Clear session manager
	session.SessionMgr = nil

	log.Util().Info("Client package shutdown complete")
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