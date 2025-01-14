package negentropy

import (
	"encoding/json"
	"grain/app/src/types"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const WebSocketTimeout = 2 * time.Second // Timeout for WebSocket responses

// CheckIfUserExistsOnRelay checks if a user exists on the relay by their pubkey.
func CheckIfUserExistsOnRelay(pubKey string, relays []string) (bool, error) {
	for _, url := range relays {
		// Connect to the relay's WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("Failed to connect to relay WebSocket: %v\n", err)
			return false, err
		}
		defer conn.Close()

		// Create a subscription filter to query for events by the pubkey
		filter := types.SubscriptionFilter{
			Authors: []string{pubKey}, // Filter by the author (pubkey)
		}

		subRequest := []interface{}{
			"REQ",
			"sub_check_user", // Unique subscription identifier
			filter,
		}

		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.Printf("Failed to marshal subscription request: %v\n", err)
			return false, err
		}

		// Send the subscription request
		if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
			log.Printf("Failed to send subscription request: %v\n", err)
			return false, err
		}

		// Channels for response handling
		msgChan := make(chan []byte)
		errChan := make(chan error)

		// Goroutine to listen for WebSocket responses
		go func() {
			_, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- err
			} else {
				msgChan <- message
			}
		}()

		// Wait for response or timeout
		select {
		case message := <-msgChan:
			var response []interface{}
			if err := json.Unmarshal(message, &response); err != nil {
				log.Printf("Failed to unmarshal response: %v\n", err)
				return false, err
			}

			// Look for "EVENT" messages indicating user existence
			if len(response) > 0 && response[0] == "EVENT" {
				log.Printf("User exists: Found event from pubkey %s\n", pubKey)
				return true, nil
			} else if len(response) > 0 && response[0] == "EOSE" {
				log.Printf("No events found for pubkey %s\n", pubKey)
				return false, nil
			}

		case err := <-errChan:
			log.Printf("Error reading WebSocket message: %v\n", err)
			return false, err

		case <-time.After(WebSocketTimeout):
			log.Printf("WebSocket response timeout for pubkey %s\n", pubKey)
			return false, nil
		}
	}
	return false, nil
}
