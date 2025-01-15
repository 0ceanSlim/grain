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

		subID := "sub_check_user" // Unique subscription identifier
		subRequest := []interface{}{
			"REQ",
			subID,
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
		done := make(chan struct{})
		eventReceived := false // Track if any events are received
		loggedOnce := false    // Avoid logging multiple times

		// Goroutine to listen for WebSocket responses
		go func() {
			defer close(done) // Signal that the goroutine is finished
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				msgChan <- message
			}
		}()

		// Process messages until EOSE is received
		for {
			select {
			case message := <-msgChan:
				var response []interface{}
				if err := json.Unmarshal(message, &response); err != nil {
					log.Printf("Failed to unmarshal response: %v\n", err)
					return false, err
				}

				// Process response
				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						eventReceived = true
						if !loggedOnce {
							log.Printf("User exists: Found events for pubkey %s\n", pubKey)
							loggedOnce = true
						}

					case "EOSE":
						log.Printf("End of subscription signal received for pubkey %s\n", pubKey)

						// Send CLOSE message
						closeRequest := []interface{}{"CLOSE", subID}
						closeJSON, err := json.Marshal(closeRequest)
						if err != nil {
							log.Printf("Failed to marshal CLOSE message: %v\n", err)
							return false, err
						}

						if err := conn.WriteMessage(websocket.TextMessage, closeJSON); err != nil {
							log.Printf("Failed to send CLOSE message: %v\n", err)
							return false, err
						}

						// Wait for CLOSED response
						select {
						case closedMsg := <-msgChan:
							var closedResponse []interface{}
							if err := json.Unmarshal(closedMsg, &closedResponse); err != nil {
								log.Printf("Failed to unmarshal CLOSED response: %v\n", err)
								return false, err
							}

							if len(closedResponse) > 0 && closedResponse[0] == "CLOSED" {
								log.Printf("Subscription closed successfully: %s\n", subID)
								return !eventReceived, nil // Return true if no events were received (new user)
							}

						case err := <-errChan:
							log.Printf("Error waiting for CLOSED response: %v\n", err)
							return false, err

						case <-time.After(WebSocketTimeout):
							log.Printf("Timeout waiting for CLOSED response for subscription %s\n", subID)
							return false, nil
						}

					default:
						log.Printf("Unexpected response: %v\n", response)
					}
				}

			case err := <-errChan:
				if err.Error() == "EOF" {
					log.Printf("Connection closed cleanly by remote host.")
					return !eventReceived, nil // Gracefully handle EOF
				}
				log.Printf("Error reading WebSocket message: %v\n", err)
				return false, err

			case <-time.After(WebSocketTimeout):
				log.Printf("WebSocket response timeout for pubkey %s\n", pubKey)
				return false, nil
			}
		}
	}
	return true, nil // Default to new user if no relays respond
}