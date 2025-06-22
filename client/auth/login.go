package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Global session manager instance
var SessionMgr *SessionManager

// Global core client instance
var coreClient *core.Client

// Application relays for initial discovery
var appRelays []string

// CreateUserSession creates a new user session with comprehensive initialization
func CreateUserSession(w http.ResponseWriter, req SessionInitRequest) (*UserSession, error) {
	if SessionMgr == nil {
		return nil, &SessionError{Message: "session manager not initialized"}
	}
	
	if coreClient == nil {
		return nil, &SessionError{Message: "core client not initialized"}
	}

	log.Util().Info("Creating user session", 
		"pubkey", req.PublicKey,
		"mode", req.RequestedMode,
		"signing_method", req.SigningMethod)

	// Get or fetch user data
	metadata, mailboxes, err := getUserDataForSession(req.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data: %w", err)
	}

	// Prepare session metadata
	sessionMetadata := SessionMetadata{}
	
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			log.Util().Warn("Failed to marshal metadata", "pubkey", req.PublicKey, "error", err)
		} else {
			sessionMetadata.Profile = string(metadataBytes)
		}
	}

	if mailboxes != nil {
		mailboxBytes, err := json.Marshal(mailboxes)
		if err != nil {
			log.Util().Warn("Failed to marshal mailboxes", "pubkey", req.PublicKey, "error", err)
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
		session.ConnectedRelays = appRelays
	}

	log.Util().Info("User session created successfully", 
		"pubkey", req.PublicKey,
		"mode", session.Mode,
		"relay_count", len(session.ConnectedRelays))

	return session, nil
}

// getUserDataForSession retrieves user metadata and mailboxes, using cache when possible
func getUserDataForSession(publicKey string) (*nostr.Event, *core.Mailboxes, error) {
	// Try cached data first using the  cache function
	if metadata, mailboxes, found := cache.GetParsedUserData(publicKey); found {
		log.Util().Debug("Using cached data for session", "pubkey", publicKey)
		return metadata, mailboxes, nil
	}
	
	// Fetch fresh data using the helper function
	log.Util().Debug("Cache miss, fetching fresh data for session", "pubkey", publicKey)
	
	// Use the comprehensive fetch function that was moved to helpers
	if err := FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
		log.Util().Warn("Failed to fetch and cache user data", "pubkey", publicKey, "error", err)
		return nil, nil, err
	}
	
	// Now get the freshly cached data
	metadata, mailboxes, found := cache.GetParsedUserData(publicKey)
	if !found {
		return nil, nil, fmt.Errorf("failed to retrieve cached data after fetch")
	}
	
	return metadata, mailboxes, nil
}

// cacheUserData stores user data in the cache (DEPRECATED - use cache.CacheUserDataFromObjects)
func cacheUserData(publicKey string, metadata *nostr.Event, mailboxes *core.Mailboxes) {
	// Use the  cache function
	cache.CacheUserDataFromObjects(publicKey, metadata, mailboxes)
}

// RebuildCacheForSession rebuilds cache for an existing session
func RebuildCacheForSession(session *UserSession) {
	log.Util().Info("Rebuilding cache for existing session", "pubkey", session.PublicKey)
	
	go func() {
		// Fetch data in background to avoid blocking
		metadata, mailboxes, err := fetchFreshUserData(session.PublicKey)
		if err != nil {
			log.Util().Error("Failed to rebuild cache for session", 
				"pubkey", session.PublicKey, "error", err)
			return
		}
		
		// Update cache
		cacheUserData(session.PublicKey, metadata, mailboxes)
		
		// Update session metadata
		sessionMetadata := SessionMetadata{}
		if metadata != nil {
			if metadataBytes, err := json.Marshal(metadata); err == nil {
				sessionMetadata.Profile = string(metadataBytes)
			}
		}
		if mailboxes != nil {
			if mailboxBytes, err := json.Marshal(mailboxes); err == nil {
				sessionMetadata.Mailboxes = string(mailboxBytes)
			}
		}
		
		// Get session token for update (this is a simplified approach)
		// In practice, you'd need to track token->session mapping
		log.Util().Info("Cache rebuilt successfully for session", "pubkey", session.PublicKey)
	}()
}

// fetchFreshUserData fetches fresh user data from relays (DEPRECATED - use FetchAndCacheUserDataWithCoreClient)
func fetchFreshUserData(publicKey string) (*nostr.Event, *core.Mailboxes, error) {
	// Use the comprehensive helper function
	if err := FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
		return nil, nil, err
	}
	
	// Get the freshly cached data
	metadata, mailboxes, found := cache.GetParsedUserData(publicKey)
	if !found {
		return nil, nil, fmt.Errorf("failed to retrieve cached data after fetch")
	}
	
	return metadata, mailboxes, nil
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