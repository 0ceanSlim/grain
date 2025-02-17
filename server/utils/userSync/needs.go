package userSync

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	config "grain/config/types"
	nostr "grain/server/types"
)

// fetchNeeds fetches all events authored by the user from the provided relays concurrently.
func fetchNeeds(pubKey string, relays []string, syncConfig config.UserSyncConfig) []nostr.Event {
	eventMap := make(map[string]nostr.Event)
	var mu sync.Mutex
	var wg sync.WaitGroup

	filter := generateUserSyncFilter(pubKey, syncConfig)

	for _, relay := range relays {
		wg.Add(1)
		go func(relay string) {
			defer wg.Done()

			conn, _, err := websocket.DefaultDialer.Dial(relay, nil)
			if err != nil {
				log.Printf("Failed to connect to relay %s: %v", relay, err)
				return
			}
			defer conn.Close()

			subRequest := []interface{}{"REQ", "sub_outbox", filter}
			requestJSON, _ := json.Marshal(subRequest)
			_ = conn.WriteMessage(websocket.TextMessage, requestJSON)

			eoseReceived := false
			for !eoseReceived {
				conn.SetReadDeadline(time.Now().Add(WebSocketTimeout))
				_, message, err := conn.ReadMessage()
				if err != nil {
					break
				}

				var response []interface{}
				_ = json.Unmarshal(message, &response)

				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						var event nostr.Event
					
						eventMap, ok := response[2].(map[string]interface{})
						if !ok {
							log.Printf("[ERROR] Unexpected event data format from relay: %+v", response[2])
							continue
						}
					
						// Convert map to JSON and unmarshal into event struct
						eventJSON, err := json.Marshal(eventMap)
						if err != nil {
							log.Printf("[ERROR] Failed to marshal event data: %v", err)
							continue
						}
					
						err = json.Unmarshal(eventJSON, &event)
						if err != nil {
							log.Printf("[ERROR] Failed to parse event from relay: %v", err)
							continue
						}
					
						mu.Lock()
						eventMap[event.ID] = event
						mu.Unlock()
					
					case "EOSE":
						eoseReceived = true
					}
				}
			}
		}(relay)
	}

	wg.Wait()
	allEvents := make([]nostr.Event, 0, len(eventMap))
	for _, evt := range eventMap {
		allEvents = append(allEvents, evt)
	}

	return allEvents
}

