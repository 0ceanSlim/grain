package negentropy

//
import (
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"grain/app/src/types"
	nostr "grain/server/types"
)

// fetchAllUserEvents fetches all events authored by the user from the provided relays concurrently.
func fetchAllUserEvents(pubKey string, relays []string) []nostr.Event {
	var (
		allEvents []nostr.Event
		mu        sync.Mutex // Protects access to `allEvents`
		wg        sync.WaitGroup
	)

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

			// Create a subscription request to fetch all events by the author (pubKey)
			filter := types.SubscriptionFilter{
				Authors: []string{pubKey},
			}
			subRequest := []interface{}{
				"REQ",
				"sub_outbox", // Unique subscription ID
				filter,
			}

			requestJSON, err := json.Marshal(subRequest)
			if err != nil {
				log.Printf("Failed to marshal subscription request for relay %s: %v", relay, err)
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
				log.Printf("Failed to send subscription request to relay %s: %v", relay, err)
				return
			}

			var relayEvents []nostr.Event

		outer:
			for {
				conn.SetReadDeadline(time.Now().Add(WebSocketTimeout)) // Set a timeout for each read operation
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
						relayEvents = append(relayEvents, event)

					case "EOSE":
						// End of subscription signal
						log.Printf("EOSE received from relay: %s", relay)
						_ = conn.WriteMessage(websocket.TextMessage, []byte(`["CLOSE", "sub_outbox"]`))
						break outer
					}
				}
			}

			// Append relayEvents to allEvents
			mu.Lock()
			allEvents = append(allEvents, relayEvents...)
			mu.Unlock()
		}(relay)
	}

	wg.Wait()

	// Sort events by created_at timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].CreatedAt < allEvents[j].CreatedAt
	})

	return allEvents
}