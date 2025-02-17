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

// triggerUserSync fetches Kind 10002 events and starts syncing missing events.
func triggerUserSync(pubKey string, userSyncCfg *configTypes.UserSyncConfig, serverCfg *configTypes.ServerConfig) {
	log.Printf("Starting user sync for pubkey: %s", pubKey)

	initialRelays := userSyncCfg.InitialSyncRelays
	if len(initialRelays) == 0 {
		log.Println("No initial relays configured for user sync.")
		return
	}

	// Fetch user's outbox events
	userOutboxEvents := fetchUserOutboxes(pubKey, initialRelays)
	if len(userOutboxEvents) == 0 {
		log.Printf("No Kind 10002 events found for pubkey: %s", pubKey)
		return
	}

	// Sort by `created_at` descending and pick the latest event
	sort.Slice(userOutboxEvents, func(i, j int) bool {
		return userOutboxEvents[i].CreatedAt > userOutboxEvents[j].CreatedAt
	})
	latestOutboxEvent := userOutboxEvents[0]

	log.Printf("Selected latest Kind 10002 event: ID=%s, CreatedAt=%d", latestOutboxEvent.ID, latestOutboxEvent.CreatedAt)

	// Store the latest outbox event in the local relay
	err := storeUserOutboxes(latestOutboxEvent, serverCfg)
	if err != nil {
		log.Printf("Failed to store Kind 10002 event to local relay: %v", err)
		return
	}

	// Extract relay URLs from event tags
	var userOutboxes []string
	for _, tag := range latestOutboxEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			userOutboxes = append(userOutboxes, tag[1])
		}
	}

	if len(userOutboxes) == 0 {
		log.Printf("No outbox relays found for pubkey: %s", pubKey)
		return
	}

	// Fetch haves & needs (wait for EOSE)
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)
	haves, err := fetchHaves(pubKey, localRelayURL, *userSyncCfg)
	if err != nil {
		log.Printf("Failed to fetch haves: %v", err)
		return
	}

	needs := fetchNeeds(pubKey, userOutboxes, *userSyncCfg)

	// Sort before comparing
	sort.Slice(haves, func(i, j int) bool { return haves[i].ID < haves[j].ID })
	sort.Slice(needs, func(i, j int) bool { return needs[i].ID < needs[j].ID })

	// Identify missing events
	missingEvents := findMissingEvents(haves, needs)
	log.Printf("Identified %d missing events.", len(missingEvents))

	// Send missing events in batches
	batchAndSendEvents(missingEvents, serverCfg)
}

// findMissingEvents compares 'have' and 'need' event lists by ID.
func findMissingEvents(haves, needs []nostr.Event) []nostr.Event {
	// Convert haves to a set for quick lookups
	haveIDs := make(map[string]struct{}, len(haves))
	for _, evt := range haves {
		haveIDs[evt.ID] = struct{}{}
	}

	// Identify missing events without duplicates
	missingSet := make(map[string]nostr.Event)

	for _, evt := range needs {
		if _, exists := haveIDs[evt.ID]; !exists {
			missingSet[evt.ID] = evt
		}
	}

	// Convert to slice
	missing := make([]nostr.Event, 0, len(missingSet))
	for _, evt := range missingSet {
		missing = append(missing, evt)
	}

	log.Printf("Missing events to store: %d (Needs: %d, Haves: %d)", len(missing), len(needs), len(haves))
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
			log.Printf("[ERROR] Failed to marshal event: %v", err)
			continue
		}

		// Write event to WebSocket (locked to prevent race conditions)
		mu.Lock()
		err = conn.WriteMessage(websocket.TextMessage, messageJSON)
		mu.Unlock()

		if err != nil {
			log.Printf("[ERROR] Failed to send event: %v", err)
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"event": event,
			})
			continue
		}

		// Read response
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ERROR] Failed to read relay response: %v", err)
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"event": event,
			})
			continue
		}

		// Parse response
		var response []interface{}
		if err := json.Unmarshal(message, &response); err != nil || len(response) < 3 {
			log.Printf("[ERROR] Invalid response format: %s", string(message))
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"event":    event,
				"response": string(message),
			})
			continue
		}

		// Log response along with event
		failedEventsLog = append(failedEventsLog, map[string]interface{}{
			"event":    event,
			"response": response,
		})

		// Check if the relay accepted the event
		if ok, okCast := response[2].(bool); okCast && ok {
			successCount++
		} else {
			failureCount++
		}
	}

	return successCount, failureCount, failedEventsLog
}


// writeFailuresToFile logs all failed events to a file in JSON format.
func writeFailuresToFile(failedEventsLog []map[string]interface{}) {
	filePath := "failed.log"
	maxSize := int64(5 * 1024 * 1024) // 5MB

	logFile, _ := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer logFile.Close()

	for _, failure := range failedEventsLog {
		failureJSON, _ := json.Marshal(failure)
		_, _ = logFile.WriteString(string(failureJSON) + "\n")
	}

	// Trim file if too large
	fileInfo, _ := logFile.Stat()
	if fileInfo.Size() > maxSize {
		_ = os.Truncate(filePath, maxSize/2) // Remove oldest half
	}
}
