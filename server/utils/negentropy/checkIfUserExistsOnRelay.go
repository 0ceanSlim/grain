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
func CheckIfUserExistsOnRelay(pubKey, eventID string, relays []string) (bool, error) {
	for _, url := range relays {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("Failed to connect to relay WebSocket: %v\n", err)
			return false, err
		}
		defer conn.Close()

		// Create a subscription filter
		filter := types.SubscriptionFilter{
			Authors: []string{pubKey},
		}
		subID := "sub_check_user"
		subRequest := []interface{}{"REQ", subID, filter}

		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.Printf("Failed to marshal subscription request: %v\n", err)
			return false, err
		}

		if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
			log.Printf("Failed to send subscription request: %v\n", err)
			return false, err
		}

		msgChan := make(chan []byte)
		errChan := make(chan error)
		eventCount := 0
		isNewUser := true

		// Goroutine for reading WebSocket messages
		go func() {
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				msgChan <- message
			}
		}()

	outer:
		for {
			select {
			case msg := <-msgChan:
				var response []interface{}
				if err := json.Unmarshal(msg, &response); err != nil {
					log.Printf("Failed to unmarshal response: %v\n", err)
					continue
				}

				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						// Parse the event
						var event types.NostrEvent
						eventData, _ := json.Marshal(response[2])
						if err := json.Unmarshal(eventData, &event); err != nil {
							log.Printf("Failed to parse event: %v\n", err)
							continue
						}

						// Increment event count
						eventCount++

						// Skip the current event being processed
						if event.ID == eventID {
							continue
						}

						// If any other event exists, it's not a new user
						isNewUser = false

					case "EOSE":
						log.Printf("EOSE received for pubkey %s\n", pubKey)
						break outer
					}
				}
			case err := <-errChan:
				log.Printf("Error reading WebSocket message: %v\n", err)
				return false, err
			case <-time.After(WebSocketTimeout):
				log.Printf("WebSocket response timeout for pubkey %s\n", pubKey)
				break outer
			}
		}

		// Close subscription
		closeRequest := []interface{}{"CLOSE", subID}
		closeJSON, err := json.Marshal(closeRequest)
		if err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, closeJSON)
		}

		// New user if only the current event exists
		return isNewUser, nil
	}
	return true, nil // Assume new user if no relays respond
}
