package handlers

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/utils/relayurl"
	"github.com/0ceanslim/grain/server/validation"
)

// Mutex to protect auth data
var authMu sync.Mutex

// Maps to track authentication sessions. authSessions stores the
// authenticated pubkey (hex) per connection; an empty/missing entry
// means the connection has not completed NIP-42 AUTH. We need the
// pubkey itself (not just a bool) so NIP-70 can verify a protected
// event came from its author's authenticated connection.
var challenges = make(map[nostr.ClientInterface]string)
var authSessions = make(map[nostr.ClientInterface]string)

// HandleAuth processes the "AUTH" message type as defined in NIP-42
func HandleAuth(client nostr.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		log.Auth().Debug("Invalid AUTH message format")
		response.SendNotice(client, "", "Invalid AUTH message format")
		return
	}

	authData, ok := message[1].(map[string]interface{})
	if !ok {
		log.Auth().Debug("Invalid auth data format")
		response.SendNotice(client, "", "Invalid auth data format")
		return
	}

	authBytes, err := json.Marshal(authData)
	if err != nil {
		log.Auth().Error("Error marshaling auth data", "error", err)
		response.SendNotice(client, "", "Error marshaling auth data")
		return
	}

	var authEvent nostr.Event
	err = json.Unmarshal(authBytes, &authEvent)
	if err != nil {
		log.Auth().Error("Error unmarshaling auth data", "error", err)
		response.SendNotice(client, "", "Error unmarshaling auth data")
		return
	}

	err = VerifyAuthEvent(client, authEvent)
	if err != nil {
		// Pull the relay tag for the failure log so URL-mismatch
		// problems can be diagnosed from the file alone (otherwise
		// the only signal is the opaque "relay URL does not match").
		gotRelay, _ := extractTag(authEvent.Tags, "relay")
		log.Auth().Info("Auth verification failed",
			"event_id", authEvent.ID,
			"pubkey", authEvent.PubKey,
			"relay_tag", gotRelay,
			"error", err)
		response.SendOK(client, authEvent.ID, false, err.Error())
		return
	}

	// Mark the session as authenticated after successful verification
	SetAuthenticated(client, authEvent.PubKey)
	ClearChallengeForConnection(client) // Clear used challenge

	log.Auth().Info("Authentication successful", "pubkey", authEvent.PubKey)
	response.SendOK(client, authEvent.ID, true, "")
}

// VerifyAuthEvent verifies the authentication event according to NIP-42
func VerifyAuthEvent(client nostr.ClientInterface, evt nostr.Event) error {
	if evt.Kind != 22242 {
		return errors.New("invalid: event kind must be 22242")
	}

	if time.Since(time.Unix(evt.CreatedAt, 0)) > 10*time.Minute {
		return errors.New("invalid: event is too old")
	}

	challenge, err := extractTag(evt.Tags, "challenge")
	if err != nil {
		return errors.New("invalid: challenge tag missing")
	}

	relayURL, err := extractTag(evt.Tags, "relay")
	if err != nil {
		return errors.New("invalid: relay tag missing")
	}

	expectedChallenge := GetChallengeForConnection(client)
	if challenge == "" || challenge != expectedChallenge {
		return errors.New("invalid: challenge does not match or is missing")
	}

	if !relayurl.Match(relayURL, config.GetConfig().Auth.RelayURL) {
		return errors.New("invalid: relay URL does not match")
	}

	if !validation.CheckSignature(evt) {
		return errors.New("invalid: signature verification failed")
	}

	return nil
}

// extractTag extracts a specific tag from an event's tags
func extractTag(tags [][]string, key string) (string, error) {
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == key {
			return tag[1], nil
		}
	}
	return "", errors.New("tag not found")
}

// GetChallengeForConnection retrieves the challenge string for a given connection
func GetChallengeForConnection(client nostr.ClientInterface) string {
	authMu.Lock()
	defer authMu.Unlock()
	return challenges[client]
}

// SetChallengeForConnection sets the challenge string for a given connection
func SetChallengeForConnection(client nostr.ClientInterface, challenge string) {
	authMu.Lock()
	defer authMu.Unlock()
	log.Auth().Debug("Setting challenge for connection", "client", client)
	challenges[client] = challenge
}

// ClearChallengeForConnection removes the challenge for a connection
func ClearChallengeForConnection(client nostr.ClientInterface) {
	authMu.Lock()
	defer authMu.Unlock()
	delete(challenges, client)
}

// SetAuthenticated marks a connection as authenticated and records the
// authenticated pubkey.
func SetAuthenticated(client nostr.ClientInterface, pubkey string) {
	authMu.Lock()
	defer authMu.Unlock()
	authSessions[client] = pubkey
}

// IsAuthenticated checks if a connection is authenticated.
func IsAuthenticated(client nostr.ClientInterface) bool {
	authMu.Lock()
	defer authMu.Unlock()
	return authSessions[client] != ""
}

// GetAuthedPubkey returns the pubkey the connection authenticated as,
// or "" if the connection has not completed NIP-42 AUTH.
func GetAuthedPubkey(client nostr.ClientInterface) string {
	authMu.Lock()
	defer authMu.Unlock()
	return authSessions[client]
}
