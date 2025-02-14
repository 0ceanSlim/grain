package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"grain/config"
	"grain/server/handlers/response"
	relay "grain/server/types"
	"grain/server/utils"
	"sync"
	"time"
)

// Mutex to protect auth data
var authMu sync.Mutex

// Maps to track authentication sessions
var challenges = make(map[string]string)
var authSessions = make(map[relay.ClientInterface]bool)

// HandleAuth processes the "AUTH" message type as defined in NIP-42
func HandleAuth(client relay.ClientInterface, message []interface{}) {
	if !config.GetConfig().Auth.Enabled {
		fmt.Println("AUTH is disabled in the configuration")
		response.SendNotice(client, "", "AUTH is disabled")
		return
	}

	if len(message) != 2 {
		fmt.Println("Invalid AUTH message format")
		response.SendNotice(client, "", "Invalid AUTH message format")
		return
	}

	authData, ok := message[1].(map[string]interface{})
	if !ok {
		fmt.Println("Invalid auth data format")
		response.SendNotice(client, "", "Invalid auth data format")
		return
	}

	authBytes, err := json.Marshal(authData)
	if err != nil {
		fmt.Println("Error marshaling auth data:", err)
		response.SendNotice(client, "", "Error marshaling auth data")
		return
	}

	var authEvent relay.Event
	err = json.Unmarshal(authBytes, &authEvent)
	if err != nil {
		fmt.Println("Error unmarshaling auth data:", err)
		response.SendNotice(client, "", "Error unmarshaling auth data")
		return
	}

	err = VerifyAuthEvent(authEvent)
	if err != nil {
		response.SendOK(client, authEvent.ID, false, err.Error())
		return
	}

	// Mark the session as authenticated after successful verification
	SetAuthenticated(client)
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

	if !utils.CheckSignature(evt) {
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
