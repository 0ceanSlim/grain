package userSync

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
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
	haves, err := fetchHaves(pubKey, fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port), *userSyncCfg)
	if err != nil {
		log.Printf("Failed to fetch haves from local relay: %v", err)
		return
	}
	log.Printf("Fetched %d events from the local relay (haves).", len(haves))

	// Fetch "needs" from the user outbox relays
	needs := fetchNeeds(pubKey, userOutboxes, *userSyncCfg)
	log.Printf("Fetched %d events from the user outbox relays (needs).", len(needs))

	// Identify missing events
	missingEvents := findMissingEvents(haves, needs)
	log.Printf("Identified %d missing events.", len(missingEvents))

	// Batch and send missing events
	batchAndSendEvents(missingEvents, serverCfg)
}

// findMissingEvents compares 'have' and 'need' event lists by ID.
func findMissingEvents(haves, needs []nostr.Event) []nostr.Event {
	haveIDs := make(map[string]struct{})
	for _, evt := range haves {
		haveIDs[evt.ID] = struct{}{}
	}

	var missing []nostr.Event
	var kind5Events []nostr.Event
	for _, evt := range needs {
		if _, exists := haveIDs[evt.ID]; !exists {
			if evt.Kind == 5 {
				kind5Events = append(kind5Events, evt)
			} else {
				missing = append(missing, evt)
			}
		}
	}

	log.Printf("Missing events: %d (Needs: %d, Haves: %d)", len(missing)+len(kind5Events), len(needs), len(haves))
	return append(missing, kind5Events...)
}

// batchAndSendEvents batches and sends events with progress updates.
func batchAndSendEvents(events []nostr.Event, serverCfg *configTypes.ServerConfig) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	rateLimit := int(serverCfg.RateLimit.EventLimit)
	batchSize := rateLimit / 2 // Send in batches of half the rate limit

	nonKind5Events := []nostr.Event{}
	kind5Events := []nostr.Event{}

	for _, evt := range events {
		if evt.Kind == 5 {
			kind5Events = append(kind5Events, evt)
		} else {
			nonKind5Events = append(nonKind5Events, evt)
		}
	}

	processBatches(nonKind5Events, batchSize, serverCfg)
	processBatches(kind5Events, batchSize, serverCfg)
}

func processBatches(events []nostr.Event, batchSize int, serverCfg *configTypes.ServerConfig) {
	totalEvents := len(events)
	successCount := 0
	failureCount := 0
	var mu sync.Mutex

	for i := 0; i < totalEvents; i += batchSize {
		end := i + batchSize
		if end > totalEvents {
			end = totalEvents
		}
		batch := events[i:end]

		if err := sendEventsToRelay(batch, serverCfg, &successCount, &failureCount, &mu); err != nil {
			log.Printf("Failed to send batch starting at index %d: %v", i, err)
		}

		log.Printf("Progress: Sent %d/%d events", i+len(batch), totalEvents)
		time.Sleep(time.Second) // Wait 1 second between batches
	}

	log.Printf("Sending complete. Success: %d, Failed: %d", successCount, failureCount)
}

// sendEventsToRelay sends a batch of events and tracks response statuses.
func sendEventsToRelay(events []nostr.Event, serverCfg *configTypes.ServerConfig, successCount, failureCount *int, mu *sync.Mutex) error {
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)

	conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to relay WebSocket: %w", err)
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

		// Read response
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed to read relay response: %w", err)
		}

		var response []interface{}
		if err := json.Unmarshal(message, &response); err != nil || len(response) < 3 {
			log.Printf("[ERROR] Invalid response format: %s", message)
			mu.Lock()
			*failureCount++
			mu.Unlock()
			continue
		}

		ok, okCast := response[2].(bool)
		if okCast && ok {
			mu.Lock()
			*successCount++
			mu.Unlock()
		} else {
			mu.Lock()
			*failureCount++
			mu.Unlock()
		}
	}

	return nil
}
