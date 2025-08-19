package session

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
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

		// IMPORTANT: After caching user data, set client relays from cached mailboxes
		// This ensures user's preferred relays REPLACE default app relays
		if err := cache.SetUserClientRelaysFromMailboxes(req.PublicKey); err != nil {
			log.ClientSession().Warn("Failed to set client relays from mailboxes",
				"pubkey", req.PublicKey,
				"error", err)
		} else {
			log.ClientSession().Info("Client relays set from user mailboxes",
				"pubkey", req.PublicKey)

			// NEW: Switch the core client to use user's relays instead of default app relays
			if err := switchCoreClientToUserRelays(req.PublicKey); err != nil {
				log.ClientSession().Warn("Failed to switch core client to user relays",
					"pubkey", req.PublicKey,
					"error", err)
				// Don't fail session creation, but log the issue
			} else {
				log.ClientSession().Info("Core client switched to user relays",
					"pubkey", req.PublicKey)
			}
		}
	}

	// Create lightweight session (no user data stored in session)
	session, err := SessionMgr.CreateSession(w, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Set connected relays to current core client relays (which should now be user's relays)
	coreClient := connection.GetCoreClient()
	if coreClient != nil {
		session.ConnectedRelays = coreClient.GetConnectedRelays()
	} else {
		session.ConnectedRelays = connection.GetClientRelays() // fallback
	}

	log.ClientSession().Info("User session created successfully",
		"pubkey", req.PublicKey,
		"mode", session.Mode,
		"connected_relay_count", len(session.ConnectedRelays))

	return session, nil
}

// switchCoreClientToUserRelays switches the core client to use user's cached relays
func switchCoreClientToUserRelays(publicKey string) error {
	log.ClientSession().Info("Switching core client to user relays", "pubkey", publicKey)

	// Get user's cached client relays
	clientRelays, err := cache.GetUserClientRelays(publicKey)
	if err != nil || len(clientRelays) == 0 {
		log.ClientSession().Warn("No user client relays found, keeping default connections",
			"pubkey", publicKey,
			"error", err)
		// Don't switch if no user relays - keep defaults
		return nil
	}

	// Get core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		return fmt.Errorf("core client not available")
	}

	// Log what relays we're switching to
	relayURLs := []string{}
	for _, relay := range clientRelays {
		relayURLs = append(relayURLs, relay.URL)
	}
	log.ClientSession().Info("User relays to connect",
		"pubkey", publicKey,
		"relay_count", len(clientRelays),
		"relay_urls", relayURLs)

	// Convert cached relays to RelayConfig format
	var relayConfigs []core.RelayConfig
	for _, relay := range clientRelays {
		// Validate relay URL
		if relay.URL == "" {
			log.ClientSession().Warn("Skipping empty relay URL", "pubkey", publicKey)
			continue
		}

		// Ensure proper URL format
		url := relay.URL
		if !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
			// Try to fix common issues
			if strings.Contains(url, "://") {
				log.ClientSession().Warn("Invalid relay URL protocol",
					"pubkey", publicKey,
					"url", url)
				continue
			}
			// Assume wss:// if no protocol
			url = "wss://" + url
			log.ClientSession().Debug("Added wss:// prefix to relay URL",
				"original", relay.URL,
				"fixed", url)
		}

		relayConfigs = append(relayConfigs, core.RelayConfig{
			URL:   url,
			Read:  relay.Read,
			Write: relay.Write,
		})
	}

	if len(relayConfigs) == 0 {
		log.ClientSession().Warn("No valid relay configs after validation, keeping defaults",
			"pubkey", publicKey,
			"original_count", len(clientRelays))
		return nil
	}

	// Switch the core client to user's relays
	if err := coreClient.SwitchToUserRelays(relayConfigs); err != nil {
		log.ClientSession().Error("Failed to switch core client to user relays",
			"pubkey", publicKey,
			"error", err)
		return err
	}

	// Verify the switch worked
	connectedCount := len(coreClient.GetConnectedRelays())
	log.ClientSession().Info("Successfully switched core client to user relays",
		"pubkey", publicKey,
		"relay_count", len(relayConfigs),
		"connected_count", connectedCount)

	if connectedCount == 0 {
		log.ClientSession().Error("No relays connected after switch, attempting fallback to defaults",
			"pubkey", publicKey)
		// Try to restore default relays
		if err := connection.SwitchToDefaultRelays(); err != nil {
			log.ClientSession().Error("Failed to restore default relays", "error", err)
		}
		return fmt.Errorf("failed to connect to any user relays")
	}

	return nil
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
