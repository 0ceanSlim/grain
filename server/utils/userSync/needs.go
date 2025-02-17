package userSync

import (
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	config "grain/config/types"
	nostr "grain/server/types"
)

func fetchNeeds(pubKey string, relays []string, syncConfig config.UserSyncConfig) []nostr.Event {
	var (
		eventMap = make(map[string]nostr.Event) // Deduplication map
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	filter := generateUserSyncFilter(pubKey, syncConfig)

	for _, relay := range relays {
		wg.Add(1)

		go func(relay string) {
			defer wg.Done()

			log.Printf("Connecting to relay: %s", relay)
			conn, _, err := websocket.DefaultDialer.Dial(relay, nil)
			if err != nil {
				log.Printf("Failed to connect to relay %s: %v", relay, err)
				return
			}
			defer conn.Close()

			subRequest := []interface{}{"REQ", "sub_outbox", filter}
			requestJSON, err := json.Marshal(subRequest)
			if err != nil {
				log.Printf("Failed to marshal subscription request for relay %s: %v", relay, err)
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
				log.Printf("Failed to send subscription request to relay %s: %v", relay, err)
				return
			}

		outer:
			for {
				conn.SetReadDeadline(time.Now().Add(WebSocketTimeout))
				_, message, err := conn.ReadMessage()
				if err != nil {
					log.Printf("Error reading from relay %s: %v", relay, err)
					break
				}

				var response []interface{}
				if err := json.Unmarshal(message, &response); err != nil {
					log.Printf("Failed to unmarshal response from relay %s: %v", relay, err)
					continue
				}

				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						var event nostr.Event
						eventData, _ := json.Marshal(response[2])
						if err := json.Unmarshal(eventData, &event); err != nil {
							log.Printf("Failed to parse event from relay %s: %v", relay, err)
							continue
						}

						// Deduplicate events by ID
						mu.Lock()
						eventMap[event.ID] = event
						mu.Unlock()

					case "EOSE":
						log.Printf("EOSE received from relay: %s", relay)
						break outer
					}
				}
			}

			_ = conn.WriteMessage(websocket.TextMessage, []byte(`["CLOSE", "sub_outbox"]`))
		}(relay)
	}

	wg.Wait()

	// Convert map to slice
	allEvents := make([]nostr.Event, 0, len(eventMap))
	for _, evt := range eventMap {
		allEvents = append(allEvents, evt)
	}

	// Sort by created_at
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].CreatedAt < allEvents[j].CreatedAt
	})

	return allEvents
}
