package userSync

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
)

// fetchNeeds fetches all events authored by the user from the provided relays concurrently.
func fetchNeeds(pubKey string, relays []string, syncConfig cfgType.UserSyncConfig) []nostr.Event {
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
				log.Printf("[ERROR] Failed to connect to relay %s: %v", relay, err)
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
					log.Printf("[ERROR] Failed to read from relay %s: %v", relay, err)
					break
				}

				var response []interface{}
				_ = json.Unmarshal(message, &response)

				if len(response) > 0 {
					switch response[0] {
					case "EVENT":
						var event nostr.Event

						eventMapData, ok := response[2].(map[string]interface{})
						if !ok {
							log.Printf("[ERROR] Unexpected event format in needs from %s: %+v", relay, response[2])
							continue
						}

						eventJSON, err := json.Marshal(eventMapData)
						if err != nil {
							log.Printf("[ERROR] Failed to marshal event in needs from %s: %v", relay, err)
							continue
						}

						err = json.Unmarshal(eventJSON, &event)
						if err != nil {
							log.Printf("[ERROR] Failed to parse event in needs from %s: %v", relay, err)
							continue
						}

						log.Printf("[NEEDS] Relay %s received event ID: %s", relay, event.ID)

						mu.Lock()
						eventMap[event.ID] = event
						mu.Unlock()

					case "EOSE":
						log.Printf("[NEEDS] EOSE received from relay: %s", relay)
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

	log.Printf("[NEEDS] Total events received: %d", len(allEvents))
	return allEvents
}
