package userSync

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// fetchNeeds fetches all events authored by the user from the provided relays concurrently.
func fetchNeeds(pubKey string, relays []string, syncConfig cfgType.UserSyncConfig) []nostr.Event {
	eventMap := make(map[string]nostr.Event)
	var mu sync.Mutex
	var wg sync.WaitGroup

	filter := generateUserSyncFilter(pubKey, syncConfig)

	log.UserSync().Info("Starting concurrent fetch from outbox relays", 
		"pubkey", pubKey,
		"relay_count", len(relays))

	for _, relay := range relays {
		wg.Add(1)
		go func(relay string) {
			defer wg.Done()

			log.UserSync().Debug("Connecting to outbox relay", 
				"relay_url", relay, 
				"pubkey", pubKey)

			conn, err := websocket.Dial(relay, "", "http://localhost/")
			if err != nil {
				log.UserSync().Error("Failed to connect to outbox relay", 
					"error", err, 
					"relay_url", relay)
				return
			}
			defer conn.Close()

			subRequest := []interface{}{"REQ", "sub_outbox", filter}
			requestJSON, err := json.Marshal(subRequest)
			if err != nil {
				log.UserSync().Error("Failed to marshal subscription request", 
					"error", err, 
					"relay_url", relay)
				return
			}

			err = websocket.Message.Send(conn, string(requestJSON))
			if err != nil {
				log.UserSync().Error("Failed to send subscription request", 
					"error", err, 
					"relay_url", relay)
				return
			}

			// Set up channels for message handling
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
							log.UserSync().Debug("Outbox relay connection closed", "relay_url", relay)
							return
						}
						errChan <- err
						return
					}
					msgChan <- message
				}
			}()

			eoseReceived := false
			eventCount := 0

			for !eoseReceived {
				select {
				case message, ok := <-msgChan:
					if !ok {
						log.UserSync().Debug("Message channel closed", "relay_url", relay)
						eoseReceived = true
						break
					}

					var response []interface{}
					if err := json.Unmarshal([]byte(message), &response); err != nil {
						log.UserSync().Error("Failed to unmarshal response", 
							"error", err, 
							"relay_url", relay,
							"raw_message", message)
						continue
					}

					if len(response) > 0 {
						switch response[0] {
						case "EVENT":
							var event nostr.Event

							eventMapData, ok := response[2].(map[string]interface{})
							if !ok {
								log.UserSync().Error("Unexpected event format in needs", 
									"relay_url", relay,
									"event_data", response[2],
									"data_type", fmt.Sprintf("%T", response[2]))
								continue
							}

							eventJSON, err := json.Marshal(eventMapData)
							if err != nil {
								log.UserSync().Error("Failed to marshal event in needs", 
									"error", err, 
									"relay_url", relay)
								continue
							}

							err = json.Unmarshal(eventJSON, &event)
							if err != nil {
								log.UserSync().Error("Failed to parse event in needs", 
									"error", err, 
									"relay_url", relay)
								continue
							}

							log.UserSync().Debug("Received outbox event", 
								"relay_url", relay,
								"event_id", event.ID,
								"kind", event.Kind,
								"created_at", event.CreatedAt)

							mu.Lock()
							eventMap[event.ID] = event
							eventCount++
							mu.Unlock()

						case "EOSE":
							log.UserSync().Debug("EOSE received from outbox relay", 
								"relay_url", relay,
								"events_received", eventCount)
							eoseReceived = true
						}
					}

				case <-time.After(WebSocketTimeout):
					log.UserSync().Warn("Timeout waiting for outbox relay response", 
						"relay_url", relay,
						"timeout_seconds", int(WebSocketTimeout.Seconds()),
						"events_received", eventCount)
					eoseReceived = true

				case err, ok := <-errChan:
					if !ok {
						log.UserSync().Debug("Error channel closed", "relay_url", relay)
						eoseReceived = true
						break
					}
					log.UserSync().Error("Error reading from outbox relay", 
						"error", err, 
						"relay_url", relay)
					eoseReceived = true
				}
			}

			// Send close message
			closeMsg := `["CLOSE", "sub_outbox"]`
			_ = websocket.Message.Send(conn, closeMsg)

		}(relay)
	}

	wg.Wait()

	allEvents := make([]nostr.Event, 0, len(eventMap))
	for _, evt := range eventMap {
		allEvents = append(allEvents, evt)
	}

	log.UserSync().Info("Finished fetching needs from all outbox relays", 
		"pubkey", pubKey,
		"total_unique_events", len(allEvents),
		"relays_queried", len(relays))

	return allEvents
}