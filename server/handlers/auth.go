package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/validation"
)

// Set the logging component for AUTH handler
func authLog() *slog.Logger {
	return utils.GetLogger("auth-handler")
}

// Mutex to protect auth data
var authMu sync.Mutex

// Maps to track authentication sessions
var challenges = make(map[string]string)
var authSessions = make(map[relay.ClientInterface]bool)

// HandleAuth processes the "AUTH" message type as defined in NIP-42
func HandleAuth(client relay.ClientInterface, message []interface{}) {
	if !config.GetConfig().Auth.Enabled {
		authLog().Debug("AUTH is disabled in configuration")
		response.SendNotice(client, "", "AUTH is disabled")
		return
	}

	if len(message) != 2 {
		authLog().Debug("Invalid AUTH message format")
		response.SendNotice(client, "", "Invalid AUTH message format")
		return
	}

	authData, ok := message[1].(map[string]interface{})
	if !ok {
		authLog().Debug("Invalid auth data format")
		response.SendNotice(client, "", "Invalid auth data format")
		return
	}

	authBytes, err := json.Marshal(authData)
	if err != nil {
		authLog().Error("Error marshaling auth data", "error", err)
		response.SendNotice(client, "", "Error marshaling auth data")
		return
	}

	var authEvent relay.Event
	err = json.Unmarshal(authBytes, &authEvent)
	if err != nil {
		authLog().Error("Error unmarshaling auth data", "error", err)
		response.SendNotice(client, "", "Error unmarshaling auth data")
		return
	}

	err = VerifyAuthEvent(authEvent)
	if err != nil {
		authLog().Info("Auth verification failed", "event_id", authEvent.ID, "pubkey", authEvent.PubKey, "error", err)
		response.SendOK(client, authEvent.ID, false, err.Error())
		return
	}

	// Mark the session as authenticated after successful verification
	SetAuthenticated(client)
	authLog().Info("Authentication successful", "pubkey", authEvent.PubKey)
	response.SendOK(client, authEvent.ID, true, "")
}

// VerifyAuthEvent verifies the authentication event according to NIP-42
func VerifyAuthEvent(evt relay.Event) error {
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
	authLog().Debug("Setting challenge for connection", "pubkey", pubKey)
	challenges[pubKey] = challenge
}

// SetAuthenticated marks a connection as authenticated
func SetAuthenticated(client relay.ClientInterface) {
	authMu.Lock()
	defer authMu.Unlock()
	authSessions[client] = true
}

// IsAuthenticated checks if a connection is authenticated
func IsAuthenticated(client relay.ClientInterface) bool {
	authMu.Lock()
	defer authMu.Unlock()
	return authSessions[client]
}