package userSync

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"golang.org/x/net/websocket"
)

// triggerUserSync fetches Kind 10002 events and starts syncing missing events.
func triggerUserSync(pubKey string, userSyncCfg *cfgType.UserSyncConfig, serverCfg *cfgType.ServerConfig) {
	log.UserSync().Info("Starting user sync", "pubkey", pubKey)

	initialRelays := userSyncCfg.InitialSyncRelays
	if len(initialRelays) == 0 {
		log.UserSync().Warn("No initial relays configured for user sync")
		return
	}

	// Fetch user's outbox events
	userOutboxEvents := fetchUserOutboxes(pubKey, initialRelays)
	if len(userOutboxEvents) == 0 {
		log.UserSync().Warn("No Kind 10002 events found", "pubkey", pubKey)
		return
	}

	// Sort by `created_at` descending and pick the latest event
	sort.Slice(userOutboxEvents, func(i, j int) bool {
		return userOutboxEvents[i].CreatedAt > userOutboxEvents[j].CreatedAt
	})
	latestOutboxEvent := userOutboxEvents[0]

	log.UserSync().Info("Selected latest Kind 10002 event",
		"event_id", latestOutboxEvent.ID,
		"created_at", latestOutboxEvent.CreatedAt)

	// Store the latest outbox event in the local relay
	err := storeUserOutboxes(latestOutboxEvent, serverCfg)
	if err != nil {
		log.UserSync().Error("Failed to store Kind 10002 event to local relay", "error", err)
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
		log.UserSync().Warn("No outbox relays found", "pubkey", pubKey)
		return
	}

	// Fetch haves & needs (wait for EOSE)
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)
	haves, err := fetchHaves(pubKey, localRelayURL, *userSyncCfg)
	if err != nil {
		log.UserSync().Error("Failed to fetch haves", "error", err)
		return
	}

	needs := fetchNeeds(pubKey, userOutboxes, *userSyncCfg)

	log.UserSync().Debug("Events before sorting",
		"haves_count", len(haves),
		"needs_count", len(needs))

	sort.Slice(haves, func(i, j int) bool { return haves[i].ID < haves[j].ID })
	sort.Slice(needs, func(i, j int) bool { return needs[i].ID < needs[j].ID })

	log.UserSync().Debug("Events after sorting",
		"haves_count", len(haves),
		"needs_count", len(needs))

	// Identify missing events
	missingEvents := findMissingEvents(haves, needs)
	log.UserSync().Info("Identified missing events", "missing_count", len(missingEvents))

	// Send missing events in batches
	batchAndSendEvents(missingEvents, serverCfg)
}

// findMissingEvents compares 'have' and 'need' event lists by ID.
func findMissingEvents(haves, needs []nostr.Event) []nostr.Event {
	haveIDs := make(map[string]struct{}, len(haves))
	for _, evt := range haves {
		haveIDs[evt.ID] = struct{}{}
	}

	missingSet := make(map[string]nostr.Event)
	for _, evt := range needs {
		if _, exists := haveIDs[evt.ID]; !exists {
			missingSet[evt.ID] = evt
		}
	}

	missing := make([]nostr.Event, 0, len(missingSet))
	for _, evt := range missingSet {
		missing = append(missing, evt)
	}

	log.UserSync().Debug("Missing events analysis",
		"missing_count", len(missing),
		"needs_count", len(needs),
		"haves_count", len(haves))
	return missing
}

// batchAndSendEvents sends events in controlled batches.
func batchAndSendEvents(events []nostr.Event, serverCfg *cfgType.ServerConfig) {
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

	log.UserSync().Info("Processing events in batches",
		"batch_size", batchSize,
		"non_kind5_events", len(nonKind5Events),
		"kind5_events", len(kind5Events))

	processBatches(nonKind5Events, batchSize, serverCfg, "Non-Kind5")
	processBatches(kind5Events, batchSize, serverCfg, "Kind5")
}

// processBatches ensures batched processing of events.
func processBatches(events []nostr.Event, batchSize int, serverCfg *cfgType.ServerConfig, label string) {
	totalEvents := len(events)
	totalSuccess := 0
	totalFailure := 0
	var failedEventsLog []map[string]interface{}

	if totalEvents == 0 {
		log.UserSync().Debug("No events to process", "batch_type", label)
		return
	}

	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)
	conn, err := websocket.Dial(localRelayURL, "", "http://localhost/")
	if err != nil {
		log.UserSync().Error("Failed to connect to relay WebSocket",
			"error", err,
			"relay_url", localRelayURL)
		return
	}
	defer conn.Close()

	var mu sync.Mutex

	log.UserSync().Info("Starting batch processing",
		"batch_type", label,
		"total_events", totalEvents,
		"batch_size", batchSize)

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

		log.UserSync().Debug("Batch progress",
			"batch_type", label,
			"processed", i+len(batch),
			"total", totalEvents,
			"success_count", successCount,
			"failure_count", failureCount)

		time.Sleep(time.Second) // Rate limit delay
	}

	log.UserSync().Info("Batch processing complete",
		"batch_type", label,
		"total_success", totalSuccess,
		"total_failure", totalFailure)

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
			log.UserSync().Error("Failed to marshal event", "error", err, "event_id", event.ID)
			continue
		}

		// Write event to WebSocket (locked to prevent race conditions)
		mu.Lock()
		err = websocket.Message.Send(conn, string(messageJSON))
		mu.Unlock()

		if err != nil {
			log.UserSync().Error("Failed to send event", "error", err, "event_id", event.ID)
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"response": fmt.Sprintf("Failed to send: %v", err),
				"event":    event,
			})
			continue
		}

		// Read response
		var responseStr string
		err = websocket.Message.Receive(conn, &responseStr)
		if err != nil {
			if err == io.EOF {
				log.UserSync().Debug("Connection closed while reading response", "event_id", event.ID)
			} else {
				log.UserSync().Error("Failed to read relay response", "error", err, "event_id", event.ID)
			}
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"response": fmt.Sprintf("Failed to read response: %v", err),
				"event":    event,
			})
			continue
		}

		// Parse response
		var response []interface{}
		if err := json.Unmarshal([]byte(responseStr), &response); err != nil || len(response) < 3 {
			log.UserSync().Error("Invalid response format",
				"raw_response", responseStr,
				"event_id", event.ID,
				"parse_error", err)
			failureCount++
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"response": responseStr,
				"event":    event,
			})
			continue
		}

		// Check if the relay accepted the event
		if ok, okCast := response[2].(bool); okCast {
			if ok {
				successCount++
				log.UserSync().Debug("Event accepted by relay", "event_id", event.ID)
			} else {
				failureCount++
				// Log rejection reason if available
				reason := "unknown"
				if len(response) > 3 {
					if reasonStr, ok := response[3].(string); ok {
						reason = reasonStr
					}
				}
				log.UserSync().Warn("Event rejected by relay",
					"event_id", event.ID,
					"reason", reason)
				failedEventsLog = append(failedEventsLog, map[string]interface{}{
					"response": response,
					"event":    event,
				})
			}
		} else {
			failureCount++
			log.UserSync().Error("Unexpected response format from relay",
				"event_id", event.ID,
				"response", response)
			failedEventsLog = append(failedEventsLog, map[string]interface{}{
				"response": response,
				"event":    event,
			})
		}
	}

	return successCount, failureCount, failedEventsLog
}

// writeFailuresToFile logs all failed events to a file in JSON format.
func writeFailuresToFile(failedEventsLog []map[string]interface{}) {
	filePath := "debug.log"
	maxSize := int64(5 * 1024 * 1024) // 5MB

	logFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.UserSync().Error("Failed to open debug log file", "error", err, "file_path", filePath)
		return
	}
	defer logFile.Close()

	for _, failure := range failedEventsLog {
		failureJSON, err := json.Marshal(failure)
		if err != nil {
			log.UserSync().Error("Failed to marshal failure log entry", "error", err)
			continue
		}
		_, err = logFile.WriteString(string(failureJSON) + "\n")
		if err != nil {
			log.UserSync().Error("Failed to write failure log entry", "error", err)
		}
	}

	// Trim file if too large
	fileInfo, err := logFile.Stat()
	if err != nil {
		log.UserSync().Error("Failed to get debug log file stats", "error", err)
		return
	}

	if fileInfo.Size() > maxSize {
		err = os.Truncate(filePath, maxSize/2) // Remove oldest half
		if err != nil {
			log.UserSync().Error("Failed to truncate debug log file", "error", err)
		} else {
			log.UserSync().Info("Debug log file truncated", "file_path", filePath, "new_size_bytes", maxSize/2)
		}
	}
}
