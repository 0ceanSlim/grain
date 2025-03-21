package userSync

import (
	"encoding/json"
	"log"
	"time"

	"github.com/0ceanslim/grain/web/types"

	"github.com/gorilla/websocket"

	nostr "github.com/0ceanslim/grain/server/types"
)

// fetchUserOutboxes fetches Kind 10002 events from a set of relays.
func fetchUserOutboxes(pubKey string, relays []string) []nostr.Event {
	var events []nostr.Event

	for _, relay := range relays {
		log.Printf("Connecting to relay: %s", relay)
		conn, _, err := websocket.DefaultDialer.Dial(relay, nil)
		if err != nil {
			log.Printf("Failed to connect to relay: %v", err)
			continue
		}
		defer conn.Close()

		// Create subscription request
		filter := types.SubscriptionFilter{
			Authors: []string{pubKey},
			Kinds:   []int{10002},
		}
		subRequest := []interface{}{
			"REQ",
			"sub1",
			filter,
		}

		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.Printf("Failed to marshal subscription request: %v", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
			log.Printf("Failed to send subscription request: %v", err)
			continue
		}

		// Channels for concurrent message handling
		msgChan := make(chan []byte)
		errChan := make(chan error)

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

	outer: // Label for the outer loop
		for {
			select {
			case msg := <-msgChan:
				var response []interface{}
				if err := json.Unmarshal(msg, &response); err != nil {
					log.Printf("Failed to unmarshal response: %v", err)
					continue
				}

				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						// Parse the event
						var event nostr.Event
						eventData, _ := json.Marshal(response[2])
						if err := json.Unmarshal(eventData, &event); err != nil {
							log.Printf("Failed to parse event: %v", err)
							continue
						}
						log.Printf("Received Kind 10002 event: ID=%s from relay: %s", event.ID, relay)
						events = append(events, event)
					case "EOSE":
						// End of subscription signal
						log.Printf("EOSE received from relay: %s", relay)
						_ = conn.WriteMessage(websocket.TextMessage, []byte(`["CLOSE", "sub1"]`))
						break outer
					}
				}
			case err := <-errChan:
				log.Printf("Error reading from relay: %v", err)
				break outer
			case <-time.After(WebSocketTimeout):
				log.Printf("Timeout waiting for response from relay: %s", relay)
				break outer
			}
		}
	}

	return events
}
