package negentropy

import (
	"encoding/json"
	"grain/app/src/types"
	configTypes "grain/config/types"
	"log"
	"sort"
	"time"

	nostr "grain/server/types"

	"github.com/gorilla/websocket"
)

// triggerUserSync fetches Kind 10002 events and stores the latest one.
func triggerUserSync(pubKey string, cfg *configTypes.NegentropyConfig) {
	log.Printf("Starting user sync for pubkey: %s", pubKey)

	initialRelays := cfg.InitialSyncRelays
	if len(initialRelays) == 0 {
		log.Println("No initial relays configured for user sync.")
		return
	}

	events := fetchKind10002Events(pubKey, initialRelays)

	if len(events) == 0 {
		log.Printf("No Kind 10002 events found for pubkey: %s", pubKey)
		return
	}

	// Sort events by `created_at` descending
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})

	// Select the newest event
	latestEvent := events[0]
	log.Printf("Selected latest Kind 10002 event: ID=%s, CreatedAt=%d", latestEvent.ID, latestEvent.CreatedAt)

	// Store the event in the local relay
	err := storeEventInLocalRelay(latestEvent)
	if err != nil {
		log.Printf("Failed to store Kind 10002 event: %v", err)
		return
	}

	log.Printf("Kind 10002 event successfully stored for pubkey: %s", pubKey)

	// Trigger the next step to aggregate user outbox events
	aggregateUserOutbox(pubKey, latestEvent)
}

// fetchKind10002Events fetches Kind 10002 events from a set of relays.
func fetchKind10002Events(pubKey string, relays []string) []nostr.Event {
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

// storeEventInLocalRelay stores an event in the local relay.
func storeEventInLocalRelay(event nostr.Event) error {
	// Placeholder: Implement storing logic in your local relay
	log.Printf("Storing event with ID: %s in local relay", event.ID)
	// Example: Call your database store method here
	return nil
}

// aggregateUserOutbox starts the process of aggregating user outbox events.
func aggregateUserOutbox(pubKey string, relayEvent nostr.Event) {
	// Extract relay URLs from the tags of the Kind 10002 event
	var relayURLs []string
	for _, tag := range relayEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			relayURLs = append(relayURLs, tag[1])
		}
	}

	log.Printf("Triggering aggregation of user outbox events for pubkey: %s from relays: %v", pubKey, relayURLs)
	// Placeholder: Implement logic for aggregating user outbox events
}
