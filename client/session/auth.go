package session

import (
	"fmt"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/data"
	"github.com/0ceanslim/grain/server/utils/log"
)

// CreateUserSession creates a new user session and ensures user data is cached
func CreateUserSession(w http.ResponseWriter, req SessionInitRequest) (*UserSession, error) {
	if SessionMgr == nil {
		return nil, &SessionError{Message: "session manager not initialized"}
	}

	if connection.GetCoreClient() == nil {
		return nil, &SessionError{Message: "core client not initialized"}
	}

	log.ClientSession().Info("Creating user session",
		"pubkey", req.PublicKey,
		"mode", req.RequestedMode,
		"signing_method", req.SigningMethod)

	// Ensure user data is cached (this populates metadata + mailboxes in cache)
	// This function handles fetching from Nostr network if not cached
	_, _, err := data.GetUserDataForSession(req.PublicKey)
	if err != nil {
		log.ClientSession().Warn("Failed to get user data for session, continuing anyway",
			"pubkey", req.PublicKey,
			"error", err)
		// Don't fail the session creation - user can still login without cached data
	} else {
		log.ClientSession().Info("User data cached successfully for session",
			"pubkey", req.PublicKey)
	}

	// Create lightweight session (no user data stored in session)
	session, err := SessionMgr.CreateSession(w, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Set connected relays to app relays (user-specific relays are in cache, not session)
	session.ConnectedRelays = connection.GetClientRelays()

	log.ClientSession().Info("User session created successfully",
		"pubkey", req.PublicKey,
		"mode", session.Mode,
		"app_relay_count", len(session.ConnectedRelays))

	return session, nil
}

// ValidateSessionRequest validates a session initialization request
func ValidateSessionRequest(req SessionInitRequest) error {
	if req.PublicKey == "" {
		return &SessionError{Message: "public key is required"}
	}

	// Validate public key format (basic check)
	if len(req.PublicKey) != 64 {
		return &SessionError{Message: "invalid public key format"}
	}

	// Validate mode
	if req.RequestedMode != ReadOnlyMode && req.RequestedMode != WriteMode {
		return &SessionError{Message: "invalid session mode"}
	}

	// Validate signing method for write mode
	if req.RequestedMode == WriteMode {
		validMethods := map[SigningMethod]bool{
			BrowserExtension: true,
			AmberSigning:     true,
			BunkerSigning:    true,
			EncryptedKey:     true,
		}

		if !validMethods[req.SigningMethod] {
			return &SessionError{Message: "invalid signing method for write mode"}
		}

		// If using encrypted key, private key must be provided
		if req.SigningMethod == EncryptedKey && req.PrivateKey == "" {
			return &SessionError{Message: "private key required for encrypted key signing method"}
		}
	} else {
		// Read-only mode should use NoSigning
		if req.SigningMethod == "" {
			req.SigningMethod = NoSigning
		}
	}

	return nil
}
