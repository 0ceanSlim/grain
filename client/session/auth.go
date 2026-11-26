package session

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/data"
	"github.com/0ceanslim/grain/server/utils/log"
)

// CreateUserSession creates a new user session and ensures user data is cached
func CreateUserSession(w http.ResponseWriter, r *http.Request, req SessionInitRequest) (*UserSession, error) {
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

	// Create lightweight session (no user data stored in session) FIRST so
	// login returns immediately. Fetching the user's metadata + mailboxes
	// from outbox relays is network-bound (seconds on cold relays, longer if
	// the user has no published relay list) — it must NOT block sign-in.
	// Snapshot the currently-connected relays for the session record.
	var connectedRelays []string
	if coreClient := connection.GetCoreClient(); coreClient != nil {
		connectedRelays = coreClient.GetConnectedRelays()
	} else {
		connectedRelays = connection.GetIndexRelays() // fallback
	}

	session, err := SessionMgr.CreateSession(w, r, req, connectedRelays)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// In the background: fetch + cache the user's data (deduped, so a client
	// polling /api/v1/cache shares this one fetch), then hold the user's own
	// relays open for them. All best-effort — login already succeeded and
	// the client hydrates the profile from the cache as it lands.
	go func() {
		warmups <- struct{}{}
		defer func() { <-warmups }()
		data.FetchUserDataDeduped(req.PublicKey)
		if err := cache.SetUserClientRelaysFromMailboxes(req.PublicKey); err != nil {
			log.ClientSession().Debug("No mailbox relays to set for user", "pubkey", req.PublicKey, "error", err)
			return
		}
		if err := holdUserRelays(session.UserKey(), req.PublicKey); err != nil {
			log.ClientSession().Warn("Failed to hold user relays", "pubkey", req.PublicKey, "error", err)
		}
		// The session may have ended (logout, eviction) while this ran; its
		// release then found nothing to close, so close what we just made.
		if !SessionMgr.HasUserKey(session.UserKey()) {
			connection.ReleaseUser(session.UserKey())
		}
	}()

	log.ClientSession().Info("User session created successfully",
		"pubkey", req.PublicKey,
		"mode", session.Mode,
		"connected_relay_count", len(session.ConnectedRelays))

	return session, nil
}

// warmups bounds how many logins fetch user data and hold relays at once, so
// a burst of logins queues its outbound work instead of all of it dialing out
// together.
var warmups = make(chan struct{}, 8)

// holdUserRelays keeps the user's cached client relays connected for them, on
// their own UserContext: other users' held relays are untouched.
func holdUserRelays(key, publicKey string) error {
	clientRelays, err := cache.GetUserClientRelays(publicKey)
	if err != nil || len(clientRelays) == 0 {
		log.ClientSession().Debug("No user client relays to hold", "pubkey", publicKey, "error", err)
		return nil
	}

	urls := make([]string, 0, len(clientRelays))
	for _, relay := range clientRelays {
		url := strings.TrimSpace(relay.URL)
		if url == "" {
			continue
		}
		if !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
			if strings.Contains(url, "://") {
				log.ClientSession().Warn("Invalid relay URL protocol", "pubkey", publicKey, "url", url)
				continue
			}
			url = "wss://" + url // assume wss:// if no protocol
		}
		urls = append(urls, url)
	}
	if len(urls) == 0 {
		return nil
	}

	uc := connection.UserForKey(key, publicKey)
	if uc == nil {
		return fmt.Errorf("core client not available")
	}
	if err := uc.HoldRelays(urls); err != nil {
		return err
	}
	log.ClientSession().Info("Holding user relays",
		"pubkey", publicKey, "requested", len(urls), "held", len(uc.HeldRelays()))
	return nil
}

// ValidateSessionRequest validates a session initialization request
func ValidateSessionRequest(req SessionInitRequest) error {
	if req.PublicKey == "" {
		return &SessionError{Message: "public key is required"}
	}

	// 64 lowercase hex characters, the only form a pubkey takes
	if !isHexPubkey(req.PublicKey) {
		return &SessionError{Message: "invalid public key format"}
	}

	// Validate mode
	if req.RequestedMode != ReadOnlyMode && req.RequestedMode != WriteMode {
		return &SessionError{Message: "invalid session mode"}
	}

	// Validate signing method for write mode
	if req.RequestedMode == WriteMode {
		validMethods := map[SigningMethod]bool{
			BrowserExtension:   true,
			AmberSigning:       true,
			BunkerSigning:      true,
			EncryptedKey:       true,
			GoogleSigning:      true,
			PomegranateSigning: true,
		}

		if !validMethods[req.SigningMethod] {
			return &SessionError{Message: "invalid signing method for write mode"}
		}
	} else {
		// Read-only mode should use NoSigning
		if req.SigningMethod == "" {
			req.SigningMethod = NoSigning
		}
	}

	return nil
}

// isHexPubkey reports whether s is 64 lowercase hex characters.
func isHexPubkey(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
