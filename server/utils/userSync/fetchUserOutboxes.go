package userSync

import (
	"encoding/json"
	"io"
	"time"

	"golang.org/x/net/websocket"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// fetchUserOutboxes fetches Kind 10002 events from a set of relays.
func fetchUserOutboxes(pubKey string, relays []string) []nostr.Event {
	var events []nostr.Event

	for _, relay := range relays {
		log.UserSync().Debug("Connecting to relay for outbox events", 
			"relay_url", relay, 
			"pubkey", pubKey)

		conn, err := websocket.Dial(relay, "", "http://localhost/")
		if err != nil {
			log.UserSync().Error("Failed to connect to relay", 
				"error", err, 
				"relay_url", relay)
			continue
		}
		defer conn.Close()

		// Create subscription request
		filter := nostr.Filter{
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
			log.UserSync().Error("Failed to marshal subscription request", 
				"error", err, 
				"relay_url", relay)
			continue
		}

		err = websocket.Message.Send(conn, string(requestJSON))
		if err != nil {
			log.UserSync().Error("Failed to send subscription request", 
				"error", err, 
				"relay_url", relay)
			continue
		}

		// Channels for concurrent message handling
		msgChan := make(chan string)
		errChan := make(chan error)

		// Goroutine for reading WebSocket messages
		go func() {
			defer close(msgChan)
			defer close(errChan)
			for {
				var message string
				err := websocket.Message.Receive(conn, &message)
				if err != nil {
					if err == io.EOF {
						log.UserSync().Debug("WebSocket connection closed", "relay_url", relay)
						return
					}
					errChan <- err
					return
				}
				msgChan <- message
			}
		}()

	outer: // Label for the outer loop
		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					log.UserSync().Debug("Message channel closed", "relay_url", relay)
					break outer
				}

				var response []interface{}
				if err := json.Unmarshal([]byte(msg), &response); err != nil {
					log.UserSync().Error("Failed to unmarshal response", 
						"error", err, 
						"relay_url", relay,
						"raw_message", msg)
					continue
				}

				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						// Parse the event
						var event nostr.Event
						eventData, err := json.Marshal(response[2])
						if err != nil {
							log.UserSync().Error("Failed to marshal event data", 
								"error", err, 
								"relay_url", relay)
							continue
						}
						
						if err := json.Unmarshal(eventData, &event); err != nil {
							log.UserSync().Error("Failed to parse event", 
								"error", err, 
								"relay_url", relay)
							continue
						}
						
						log.UserSync().Debug("Received Kind 10002 event", 
							"event_id", event.ID, 
							"relay_url", relay,
							"created_at", event.CreatedAt)
						events = append(events, event)

					case "EOSE":
						// End of subscription signal
						log.UserSync().Debug("EOSE received", "relay_url", relay)
						closeMsg := `["CLOSE", "sub1"]`
						_ = websocket.Message.Send(conn, closeMsg)
						break outer
					}
				}

			case err, ok := <-errChan:
				if !ok {
					log.UserSync().Debug("Error channel closed", "relay_url", relay)
					break outer
				}
				log.UserSync().Error("Error reading from relay", 
					"error", err, 
					"relay_url", relay)
				break outer

			case <-time.After(WebSocketTimeout):
				log.UserSync().Warn("Timeout waiting for response from relay", 
					"relay_url", relay,
					"timeout_seconds", int(WebSocketTimeout.Seconds()))
				break outer
			}
		}
	}

	log.UserSync().Info("Finished fetching outbox events", 
		"pubkey", pubKey, 
		"total_events", len(events),
		"relays_queried", len(relays))

	return events
}