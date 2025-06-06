package core

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/0ceanslim/grain/client/core/helpers"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PublishEvent publishes an event to a list of relays
func PublishEvent(event nostr.Event, relays []string) error {
	log.Util().Debug("Publishing event", 
		"event_id", event.ID,
		"kind", event.Kind,
		"relay_count", len(relays))

	var lastErr error
	published := 0

	for _, url := range relays {
		if err := publishToRelay(event, url); err != nil {
			log.Util().Warn("Failed to publish to relay", 
				"relay", url, 
				"event_id", event.ID, 
				"error", err)
			lastErr = err
		} else {
			published++
			log.Util().Debug("Successfully published to relay", 
				"relay", url, 
				"event_id", event.ID)
		}
	}

	if published == 0 {
		return fmt.Errorf("failed to publish to any relay: %w", lastErr)
	}

	log.Util().Info("Event published successfully", 
		"event_id", event.ID,
		"published_count", published,
		"total_relays", len(relays))

	return nil
}

// publishToRelay publishes an event to a single relay
func publishToRelay(event nostr.Event, relayURL string) error {
	conn, err := helpers.DialWithTimeout(relayURL, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Create EVENT message
	eventMessage := []interface{}{"EVENT", event}
	messageJSON, err := json.Marshal(eventMessage)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// Send event
	if _, err := conn.Write(messageJSON); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}

	// Wait for OK response
	message, err := helpers.ReadMessageWithTimeout(conn, 5*time.Second)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	var response []interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		return fmt.Errorf("parse response failed: %w", err)
	}

	// Check OK response format: ["OK", <event_id>, <true|false>, <message>]
	if len(response) >= 3 && response[0] == "OK" {
		if accepted, ok := response[2].(bool); ok && accepted {
			return nil
		}
		
		// Event was rejected
		reason := "unknown reason"
		if len(response) >= 4 {
			if msg, ok := response[3].(string); ok {
				reason = msg
			}
		}
		return fmt.Errorf("event rejected: %s", reason)
	}

	return fmt.Errorf("unexpected response format: %v", response)
}

// QueryEvents queries events from a list of relays with the given filter
func QueryEvents(filter nostr.Filter, relays []string, timeout time.Duration) ([]nostr.Event, error) {
	log.Util().Debug("Querying events", 
		"relay_count", len(relays),
		"timeout", timeout)

	var allEvents []nostr.Event
	eventMap := make(map[string]nostr.Event) // Deduplicate by event ID

	for _, url := range relays {
		events, err := queryFromRelay(filter, url, timeout)
		if err != nil {
			log.Util().Warn("Failed to query relay", "relay", url, "error", err)
			continue
		}

		// Add events to map for deduplication
		for _, event := range events {
			eventMap[event.ID] = event
		}
	}

	// Convert map to slice
	for _, event := range eventMap {
		allEvents = append(allEvents, event)
	}

	log.Util().Info("Query completed", 
		"total_events", len(allEvents),
		"relays_queried", len(relays))

	return allEvents, nil
}

// queryFromRelay queries events from a single relay
func queryFromRelay(filter nostr.Filter, relayURL string, timeout time.Duration) ([]nostr.Event, error) {
	conn, err := helpers.DialWithTimeout(relayURL, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	subscriptionID := "query-events"

	// Send REQ
	subRequest := []interface{}{"REQ", subscriptionID, filter}
	requestJSON, err := json.Marshal(subRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}

	if _, err := conn.Write(requestJSON); err != nil {
		return nil, fmt.Errorf("send failed: %w", err)
	}

	var events []nostr.Event
	deadline := time.Now().Add(timeout)

	// Read events until EOSE or timeout
	for time.Now().Before(deadline) {
		message, err := helpers.ReadMessageWithTimeout(conn, time.Until(deadline))
		if err != nil {
			break
		}

		var response []interface{}
		if err := json.Unmarshal(message, &response); err != nil {
			continue
		}

		switch response[0] {
		case "EVENT":
			var event nostr.Event
			eventData, _ := json.Marshal(response[2])
			if err := json.Unmarshal(eventData, &event); err != nil {
				continue
			}
			events = append(events, event)

		case "EOSE":
			helpers.SendCloseMessage(conn, subscriptionID)
			return events, nil
		}
	}

	return events, nil
}