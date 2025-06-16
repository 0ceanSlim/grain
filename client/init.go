// client/init.go
package client

import (
	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/server/utils/log"
)

// InitializeClient sets up the client package with core client and session management
func InitializeClient(relays []string) error {
	log.Util().Info("Initializing client package", "relay_count", len(relays))

	// Initialize session manager
	if err := initializeSessionManager(); err != nil {
		return err
	}

	// Initialize core client with relays
	if err := auth.InitializeCoreClient(relays); err != nil {
		return err
	}

	// Set app relays for discovery
	auth.SetAppRelays(relays)

	log.Util().Info("Client package initialized successfully")
	return nil
}

// initializeSessionManager sets up the session manager
func initializeSessionManager() error {
	auth.SessionMgr = auth.NewSessionManager()
	if auth.SessionMgr == nil {
		return &ClientInitError{Message: "failed to create session manager"}
	}

	log.Util().Debug("Session manager initialized")
	return nil
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
	auth.SessionMgr = nil

	log.Util().Info("Client package shutdown complete")
	return nil
}

// GetCoreClient returns the core client instance for advanced usage
func GetCoreClient() interface{} {
	return auth.GetCoreClient()
}

// ClientInitError represents initialization errors
type ClientInitError struct {
	Message string
}

func (e *ClientInitError) Error() string {
	return "client init error: " + e.Message
}