package userSync

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	configTypes "grain/config/types"
	nostr "grain/server/types"

	"github.com/gorilla/websocket"
)

// triggerUserSync fetches Kind 10002 events and stores the latest one.
func triggerUserSync(pubKey string, userSyncCfg *configTypes.UserSyncConfig, serverCfg *configTypes.ServerConfig) {
	log.Printf("Starting user sync for pubkey: %s", pubKey)

	initialRelays := userSyncCfg.InitialSyncRelays
	if len(initialRelays) == 0 {
		log.Println("No initial relays configured for user sync.")
		return
	}

	// Fetch user outboxes from the initial relays
	userOutboxEvents := fetchUserOutboxes(pubKey, initialRelays)
	if len(userOutboxEvents) == 0 {
		log.Printf("No Kind 10002 events found for pubkey: %s", pubKey)
		return
	}

	// Sort user outbox events by `created_at` descending
	sort.Slice(userOutboxEvents, func(i, j int) bool {
		return userOutboxEvents[i].CreatedAt > userOutboxEvents[j].CreatedAt
	})

	// Select the newest outbox event
	latestOutboxEvent := userOutboxEvents[0]
	log.Printf("Selected latest Kind 10002 event: ID=%s, CreatedAt=%d", latestOutboxEvent.ID, latestOutboxEvent.CreatedAt)

	// Forward the event to the local relay
	err := storeUserOutboxes(latestOutboxEvent, serverCfg)
	if err != nil {
		log.Printf("Failed to forward Kind 10002 event to local relay: %v", err)
		return
	}

	log.Printf("Kind 10002 event successfully stored for pubkey: %s", pubKey)

	// Extract user outbox relays from the tags of the latest event
	var userOutboxes []string
	for _, tag := range latestOutboxEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			userOutboxes = append(userOutboxes, tag[1])
		}
	}

	if len(userOutboxes) == 0 {
		log.Printf("No outbox relays found in the latest Kind 10002 event for pubkey: %s", pubKey)
		return
	}

	// Fetch "haves" from the local relay
	haves, err := fetchHaves(pubKey, fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port))
	if err != nil {
		log.Printf("Failed to fetch haves from local relay: %v", err)
		return
	}
	log.Printf("Fetched %d events from the local relay (haves).", len(haves))

	// Fetch "needs" from the user outbox relays
	needs := fetchNeeds(pubKey, userOutboxes)
	log.Printf("Fetched %d events from the user outbox relays (needs).", len(needs))

	// Identify missing events
	missingEvents := findMissingEvents(haves, needs)
	log.Printf("Identified %d missing events.", len(missingEvents))

	// Batch and send missing events
	batchAndSendEvents(missingEvents, serverCfg)
}

// findMissingEvents compares have and need event lists by ID.
func findMissingEvents(haves, needs []nostr.Event) []nostr.Event {
	haveIDs := make(map[string]struct{})
	for _, evt := range haves {
		haveIDs[evt.ID] = struct{}{}
	}

	var missing []nostr.Event
	for _, evt := range needs {
		if _, exists := haveIDs[evt.ID]; !exists {
			missing = append(missing, evt)
		}
	}
	return missing
}

// batchAndSendEvents batches missing events based on rate limits and sends them.
func batchAndSendEvents(events []nostr.Event, serverCfg *configTypes.ServerConfig) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	rateLimit := int(serverCfg.RateLimit.EventLimit)
	burstLimit := serverCfg.RateLimit.EventBurst

	batchSize := rateLimit
	if batchSize > burstLimit {
		batchSize = burstLimit
	}

	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		if err := sendEventsToRelay(batch, serverCfg); err != nil {
			log.Printf("Failed to send event batch: %v", err)
		}

		time.Sleep(time.Second / time.Duration(rateLimit)) // Rate limiting
	}
}

// sendEventsToRelay sends a batch of events to the relay via WebSocket.
func sendEventsToRelay(events []nostr.Event, serverCfg *configTypes.ServerConfig) error {
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)

	conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to local relay WebSocket: %w", err)
	}
	defer func() {
		closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing connection")
		if err := conn.WriteMessage(websocket.CloseMessage, closeMessage); err != nil {
			log.Printf("[ERROR] Failed to send CLOSE message: %v", err)
		}
		conn.Close()
	}()

	for _, event := range events {
		eventMessage := []interface{}{"EVENT", event}
		messageJSON, err := json.Marshal(eventMessage)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		if err := conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
			return fmt.Errorf("failed to send event: %w", err)
		}

		log.Printf("Event with ID: %s successfully sent to local relay.", event.ID)
	}

	return nil
}
