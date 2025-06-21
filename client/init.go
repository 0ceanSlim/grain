package client

import (
	"time"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/server/utils/log"
)

// InitializeClient sets up the client package with enhanced session management
func InitializeClient(relays []string) error {
	log.Util().Info("Initializing client package with enhanced session management", "relay_count", len(relays))

	// Initialize enhanced session manager
	if err := initializeEnhancedSessionManager(); err != nil {
		return err
	}

	// Initialize core client with relays
	if err := auth.InitializeCoreClient(relays); err != nil {
		return err
	}

	// Set app relays for discovery
	auth.SetAppRelays(relays)

	// Start background session cleanup
	startSessionCleanup()

	// Start cache cleanup
	startCacheCleanup()

	log.Util().Info("Client package initialized successfully with enhanced features")
	return nil
}

// initializeEnhancedSessionManager sets up the enhanced session manager
func initializeEnhancedSessionManager() error {
	auth.EnhancedSessionMgr = auth.NewEnhancedSessionManager()
	if auth.EnhancedSessionMgr == nil {
		return &ClientInitError{Message: "failed to create enhanced session manager"}
	}

	log.Util().Debug("Enhanced session manager initialized")
	return nil
}

// startSessionCleanup starts a background goroutine to clean up expired sessions
func startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute) // Clean up every 30 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if auth.EnhancedSessionMgr != nil {
					// Clean up sessions older than 24 hours of inactivity
					auth.EnhancedSessionMgr.CleanupSessions(24 * time.Hour)
					
					// Log session statistics
					stats := auth.EnhancedSessionMgr.GetSessionStats()
					log.Util().Debug("Session cleanup completed", 
						"total_sessions", stats["total_sessions"],
						"read_only", stats["read_only"],
						"write_mode", stats["write_mode"])
				}
			}
		}
	}()
	
	log.Util().Debug("Session cleanup routine started")
}

// startCacheCleanup starts a background goroutine to clean up expired cache entries
func startCacheCleanup() {
	go func() {
		ticker := time.NewTicker(15 * time.Minute) // Clean up every 15 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cache.CleanupExpired()
				log.Util().Debug("Cache cleanup completed")
			}
		}
	}()
	
	log.Util().Debug("Cache cleanup routine started")
}

// ShutdownClient gracefully shuts down the client package
func ShutdownClient() error {
	log.Util().Info("Shutting down client package")

	// Close core client connections
	if err := auth.CloseCoreClient(); err != nil {
		log.Util().Error("Error closing core client", "error", err)
		return err
	}

	// Clear session manager
	auth.EnhancedSessionMgr = nil

	log.Util().Info("Client package shutdown complete")
	return nil
}

// GetCoreClient returns the core client instance for advanced usage
func GetCoreClient() interface{} {
	return auth.GetCoreClient()
}

// GetSessionStats returns current session statistics
func GetSessionStats() map[string]interface{} {
	if auth.EnhancedSessionMgr == nil {
		return map[string]interface{}{
			"error": "session manager not initialized",
		}
	}
	
	return auth.EnhancedSessionMgr.GetSessionStats()
}

// ClientInitError represents initialization errors
type ClientInitError struct {
	Message string
}

func (e *ClientInitError) Error() string {
	return "client init error: " + e.Message
}