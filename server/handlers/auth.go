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
	"github.com/0ceanslim/grain/server/validation"
)

// Mutex to protect auth data
var authMu sync.Mutex

// Maps to track authentication sessions
var challenges = make(map[string]string)
var authSessions = make(map[nostr.ClientInterface]bool)

// HandleAuth processes the "AUTH" message type as defined in NIP-42
func HandleAuth(client nostr.ClientInterface, message []interface{}) {
	if !config.GetConfig().Auth.Enabled {
		log.Auth().Debug("AUTH is disabled in configuration")
		response.SendNotice(client, "", "AUTH is disabled")
		return
	}

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

	err = VerifyAuthEvent(authEvent)
	if err != nil {
		log.Auth().Info("Auth verification failed", "event_id", authEvent.ID, "pubkey", authEvent.PubKey, "error", err)
		response.SendOK(client, authEvent.ID, false, err.Error())
		return
	}

	// Mark the session as authenticated after successful verification
	SetAuthenticated(client)
	log.Auth().Info("Authentication successful", "pubkey", authEvent.PubKey)
	response.SendOK(client, authEvent.ID, true, "")
}

// VerifyAuthEvent verifies the authentication event according to NIP-42
func VerifyAuthEvent(evt nostr.Event) error {
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

	expectedChallenge := GetChallengeForConnection(evt.PubKey)
	if challenge != expectedChallenge {
		return errors.New("invalid: challenge does not match")
	}

	if relayURL != config.GetConfig().Auth.RelayURL {
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
func GetChallengeForConnection(pubKey string) string {
	authMu.Lock()
	defer authMu.Unlock()
	return challenges[pubKey]
}

// SetChallengeForConnection sets the challenge string for a given connection
func SetChallengeForConnection(pubKey, challenge string) {
	authMu.Lock()
	defer authMu.Unlock()
	log.Auth().Debug("Setting challenge for connection", "pubkey", pubKey)
	challenges[pubKey] = challenge
}

// SetAuthenticated marks a connection as authenticated
func SetAuthenticated(client nostr.ClientInterface) {
	authMu.Lock()
	defer authMu.Unlock()
	authSessions[client] = true
}

// IsAuthenticated checks if a connection is authenticated
func IsAuthenticated(client nostr.ClientInterface) bool {
	authMu.Lock()
	defer authMu.Unlock()
	return authSessions[client]
}