package session

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/data"
	"github.com/0ceanslim/grain/server/utils/log"
)

// CreateUserSession creates a new user session with comprehensive initialization
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

	// Get or fetch user data
	metadata, mailboxes, err := data.GetUserDataForSession(req.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data: %w", err)
	}

	// Prepare session metadata
	sessionMetadata := UserMetadata{}
	
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			log.ClientSession().Warn("Failed to marshal metadata", "pubkey", req.PublicKey, "error", err)
		} else {
			sessionMetadata.Profile = string(metadataBytes)
		}
	}

	if mailboxes != nil {
		mailboxBytes, err := json.Marshal(mailboxes)
		if err != nil {
			log.ClientSession().Warn("Failed to marshal mailboxes", "pubkey", req.PublicKey, "error", err)
		} else {
			sessionMetadata.Mailboxes = string(mailboxBytes)
		}
	}

	// Create the session
	session, err := SessionMgr.CreateSession(w, req, sessionMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update connected relays in session
	if mailboxes != nil {
		session.ConnectedRelays = mailboxes.ToStringSlice()
	} else {
		session.ConnectedRelays = connection.GetAppRelays()
	}

	log.ClientSession().Info("User session created successfully", 
		"pubkey", req.PublicKey,
		"mode", session.Mode,
		"relay_count", len(session.ConnectedRelays))

	return session, nil
}

// ValidateSessionRequest validates a session initialization request
func ValidateSessionRequest(req SessionInitRequest) error {
	if req.PublicKey == "" {
		return &SessionError{Message: "public key is required"}
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
	}
	
	return nil
}