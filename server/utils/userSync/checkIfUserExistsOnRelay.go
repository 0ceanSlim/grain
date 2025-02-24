package userSync

import (
	"encoding/json"
	"grain/web/types"
	"log"
	"time"

	"github.com/gorilla/websocket"

	nostr "grain/server/types"
)

// CheckIfUserExistsOnRelay checks if a user exists on the relay by their pubkey.
func CheckIfUserExistsOnRelay(pubKey, eventID string, relays []string) (bool, error) {
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

		for {
			select {
			case message := <-msgChan:
				var response []interface{}
				if err := json.Unmarshal(message, &response); err != nil {
					log.Printf("Failed to unmarshal response: %v\n", err)
					continue
				}

				if len(response) > 0 && response[0] == "EVENT" {
					// Parse the event
					eventData, ok := response[2].(map[string]interface{})
					if !ok {
						log.Printf("Unexpected event data type: %T\n", response[2])
						continue
					}

					// Extract the event
					eventBytes, err := json.Marshal(eventData)
					if err != nil {
						log.Printf("Failed to marshal event data: %v\n", err)
						continue
					}

					var event nostr.Event
					if err := json.Unmarshal(eventBytes, &event); err != nil {
						log.Printf("Failed to unmarshal event: %v\n", err)
						continue
					}

					// Skip the current event being processed
					if event.ID == eventID {
						continue
					}

					eventCount++
					isNewUser = false
				}

				if len(response) > 0 && response[0] == "EOSE" {
					return isNewUser, nil
				}
			case <-time.After(WebSocketTimeout):
				log.Printf("WebSocket timeout while checking user existence for pubkey: %s\n", pubKey)
				return isNewUser, nil
			case err := <-errChan:
				log.Printf("Error reading from WebSocket: %v\n", err)
				return false, err
			}
		}
	}
	return true, nil
}
