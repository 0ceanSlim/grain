package userSync

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	haveIDs := make(map[string]struct{}, len(haves))

	// Store all existing event IDs
	for _, evt := range haves {
		haveIDs[evt.ID] = struct{}{}
	}

	// Use map for deduplication
	missingMap := make(map[string]nostr.Event)

	// Identify missing events
	for _, evt := range needs {
		if _, exists := haveIDs[evt.ID]; !exists {
			missingMap[evt.ID] = evt
		}
	}

	// Convert to slice
	missing := make([]nostr.Event, 0, len(missingMap))
	for _, evt := range missingMap {
		missing = append(missing, evt)
	}

	// Sort by created_at
	sort.Slice(missing, func(i, j int) bool {
		return missing[i].CreatedAt < missing[j].CreatedAt
	})

	log.Printf("Missing events: %d (Needs: %d, Haves: %d)", len(missing), len(needs), len(haves))
	return missing
}


// batchAndSendEvents sends events in controlled batches.
func batchAndSendEvents(events []nostr.Event, serverCfg *configTypes.ServerConfig) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	rateLimit := int(serverCfg.RateLimit.EventLimit)
	batchSize := rateLimit / 2 // Half the rate limit

	nonKind5Events := []nostr.Event{}
	kind5Events := []nostr.Event{}

	for _, evt := range events {
		if evt.Kind == 5 {
			kind5Events = append(kind5Events, evt)
		} else {
			nonKind5Events = append(nonKind5Events, evt)
		}
	}

	processBatches(nonKind5Events, batchSize, serverCfg, "Non-Kind5")
	processBatches(kind5Events, batchSize, serverCfg, "Kind5")
}

// processBatches ensures batched processing of events.
func processBatches(events []nostr.Event, batchSize int, serverCfg *configTypes.ServerConfig, label string) {
	totalEvents := len(events)
	totalSuccess := 0
	totalFailure := 0
	var failedEventsLog []map[string]interface{}

	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)
	conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to connect to relay WebSocket: %v", err)
		return
	}
	defer conn.Close()

	var mu sync.Mutex

	for i := 0; i < totalEvents; i += batchSize {
		end := i + batchSize
		if end > totalEvents {
			end = totalEvents
		}
		batch := events[i:end]

		successCount, failureCount, failures := sendEventsToRelay(conn, batch, &mu)

		totalSuccess += successCount
		totalFailure += failureCount
		failedEventsLog = append(failedEventsLog, failures...)

		log.Printf("[%s] Progress: Sent %d/%d events", label, i+len(batch), totalEvents)
		time.Sleep(time.Second) // Rate limit delay
	}

	log.Printf("[%s] Sending complete. Success: %d, Failed: %d", label, totalSuccess, totalFailure)

	if totalFailure > 0 {
		writeFailuresToFile(failedEventsLog)
	}
}

// sendEventsToRelay sends a batch of events and returns success/failure counts.
func sendEventsToRelay(conn *websocket.Conn, events []nostr.Event, mu *sync.Mutex) (int, int, []map[string]interface{}) {
	successCount := 0
	failureCount := 0
	var failedEventsLog []map[string]interface{}

	for _, event := range events {
		eventMessage := []interface{}{"EVENT", event}
		messageJSON, err := json.Marshal(eventMessage)
		if err != nil {
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"error": fmt.Sprintf("Failed to marshal event: %v", err),
				"event": event,
			})
			continue
		}

		mu.Lock()
		err = conn.WriteMessage(websocket.TextMessage, messageJSON)
		mu.Unlock()

		if err != nil {
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"error": fmt.Sprintf("Failed to send event: %v", err),
				"event": event,
			})
			continue
		}

		// Read response
		_, message, err := conn.ReadMessage()
		if err != nil {
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"error": fmt.Sprintf("Failed to read relay response: %v", err),
				"event": event,
			})
			continue
		}

		var response []interface{}
		if err := json.Unmarshal(message, &response); err != nil || len(response) < 3 {
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"error":    "Invalid response format",
				"response": string(message),
				"event":    event,
			})
			continue
		}

		ok, okCast := response[2].(bool)
		if okCast && ok {
			successCount++
		} else {
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"error":    "Relay rejected event",
				"response": response,
				"event":    event,
			})
		}
	}

	return successCount, failureCount, failedEventsLog
}

// writeFailuresToFile logs all failed events to a file in JSON format.
func writeFailuresToFile(failedEventsLog []map[string]interface{}) {
	logFile, err := os.OpenFile("failed.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[ERROR] Failed to open failed.log: %v", err)
		return
	}
	defer logFile.Close()

	for _, failure := range failedEventsLog {
		failureJSON, _ := json.Marshal(failure)
		_, err := logFile.WriteString(string(failureJSON) + "\n")
		if err != nil {
			log.Printf("[ERROR] Failed to write to failed.log: %v", err)
		}
	}

	err = logFile.Sync()
	if err != nil {
		log.Printf("[ERROR] Failed to flush failed.log: %v", err)
	}
}