package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"grain/config"
	"grain/server/handlers/response"
	"grain/server/utils"
	"time"

	relay "grain/server/types"

	"golang.org/x/net/websocket"
)

// HandleAuth handles the "AUTH" message type as defined in NIP-42
func HandleAuth(ws *websocket.Conn, message []interface{}) {
	if !config.GetConfig().Auth.Enabled {
		fmt.Println("AUTH is disabled in the configuration")
		response.SendNotice(ws, "", "AUTH is disabled")
		return
	}

	if len(message) != 2 {
		fmt.Println("Invalid AUTH message format")
		response.SendNotice(ws, "", "Invalid AUTH message format")
		return
	}

	authData, ok := message[1].(map[string]interface{})
	if !ok {
		fmt.Println("Invalid auth data format")
		response.SendNotice(ws, "", "Invalid auth data format")
		return
	}
	authBytes, err := json.Marshal(authData)
	if err != nil {
		fmt.Println("Error marshaling auth data:", err)
		response.SendNotice(ws, "", "Error marshaling auth data")
		return
	}

	var authEvent relay.Event
	err = json.Unmarshal(authBytes, &authEvent)
	if err != nil {
		fmt.Println("Error unmarshaling auth data:", err)
		response.SendNotice(ws, "", "Error unmarshaling auth data")
		return
	}

	err = VerifyAuthEvent(authEvent)
	if err != nil {
		response.SendOK(ws, authEvent.ID, false, err.Error())
		return
	}

	// Mark the session as authenticated after successful verification
	SetAuthenticated(ws)
	response.SendOK(ws, authEvent.ID, true, "")
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

// Map to store challenges associated with connections
var challenges = make(map[string]string)
var authSessions = make(map[*websocket.Conn]bool)

// GetChallengeForConnection retrieves the challenge string for a given connection
func GetChallengeForConnection(pubKey string) string {
	mu.Lock()
	defer mu.Unlock()
	return challenges[pubKey]
}

// SetChallengeForConnection sets the challenge string for a given connection
func SetChallengeForConnection(pubKey, challenge string) {
	mu.Lock()
	defer mu.Unlock()
	challenges[pubKey] = challenge
}

// SetAuthenticated marks a connection as authenticated
func SetAuthenticated(ws *websocket.Conn) {
	mu.Lock()
	defer mu.Unlock()
	authSessions[ws] = true
}

// IsAuthenticated checks if a connection is authenticated
func IsAuthenticated(ws *websocket.Conn) bool {
	mu.Lock()
	defer mu.Unlock()
	return authSessions[ws]
}
