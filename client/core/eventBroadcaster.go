package core

import (
	"fmt"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// BroadcastResult represents the result of broadcasting to a single relay
type BroadcastResult struct {
	RelayURL string
	Success  bool
	Error    error
	Message  string
	Duration time.Duration
}

// BroadcastEvent sends an event to multiple relays using the relay pool
func BroadcastEvent(event *nostr.Event, relays []string, pool *RelayPool) []BroadcastResult {
	if event == nil {
		return []BroadcastResult{{
			Success: false,
			Error:   fmt.Errorf("event cannot be nil"),
			Message: "invalid event",
		}}
	}
	
	if len(relays) == 0 {
		return []BroadcastResult{{
			Success: false,
			Error:   fmt.Errorf("no relays specified"),
			Message: "no relays",
		}}
	}
	
	log.ClientCore().Info("Broadcasting event", "event_id", event.ID, "relay_count", len(relays))
	
	// Create EVENT message
	eventMessage := []interface{}{"EVENT", event}
	
	results := make([]BroadcastResult, len(relays))
	var wg sync.WaitGroup
	
	// Broadcast to each relay concurrently
	for i, relayURL := range relays {
		wg.Add(1)
		go func(index int, relay string) {
			defer wg.Done()
			
			start := time.Now()
			results[index] = broadcastToSingleRelay(relay, eventMessage, pool)
			results[index].RelayURL = relay
			results[index].Duration = time.Since(start)
		}(i, relayURL)
	}
	
	wg.Wait()
	
	// Log summary
	successful := 0
	failed := 0
	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
		}
	}
	
	log.ClientCore().Info("Broadcast completed", 
		"event_id", event.ID,
		"successful", successful,
		"failed", failed,
		"total", len(relays))
	
	return results
}

// broadcastToSingleRelay broadcasts to a single relay
func broadcastToSingleRelay(relayURL string, message []interface{}, pool *RelayPool) BroadcastResult {
	err := pool.SendMessage(relayURL, message)
	if err != nil {
		log.ClientCore().Warn("Failed to broadcast to relay", "relay", relayURL, "error", err)
		return BroadcastResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("send failed: %v", err),
		}
	}
	
	log.ClientCore().Debug("Event broadcast successful", "relay", relayURL)
	return BroadcastResult{
		Success: true,
		Message: "broadcast successful",
	}
}

// BroadcastToUserRelays broadcasts an event to a user's preferred relays
func BroadcastToUserRelays(event *nostr.Event, pubkey string, client *Client) []BroadcastResult {
	if client == nil {
		return []BroadcastResult{{
			Success: false,
			Error:   fmt.Errorf("client cannot be nil"),
			Message: "invalid client",
		}}
	}
	
	log.ClientCore().Debug("Getting user relays for broadcast", "pubkey", pubkey)
	
	// Get user's relay list
	mailboxes, err := client.GetUserRelays(pubkey)
	if err != nil {
		log.ClientCore().Warn("Failed to get user relays, using default relays", "pubkey", pubkey, "error", err)
		return BroadcastEvent(event, client.config.DefaultRelays, client.relayPool)
	}
	
	// Use write relays for broadcasting
	relays := mailboxes.Write
	if len(relays) == 0 {
		// Fall back to 'both' relays if no write-specific relays
		relays = mailboxes.Both
	}
	
	if len(relays) == 0 {
		// Fall back to default relays if user has no relay preferences
		log.ClientCore().Warn("User has no relay preferences, using default relays", "pubkey", pubkey)
		relays = client.config.DefaultRelays
	}
	
	log.ClientCore().Info("Broadcasting to user relays", "pubkey", pubkey, "relay_count", len(relays))
	return BroadcastEvent(event, relays, client.relayPool)
}

// BroadcastWithRetry broadcasts an event with retry logic
func BroadcastWithRetry(event *nostr.Event, relays []string, pool *RelayPool, maxRetries int) []BroadcastResult {
	if maxRetries < 1 {
		maxRetries = 1
	}
	
	var results []BroadcastResult
	failedRelays := make([]string, 0)
	
	log.ClientCore().Info("Broadcasting with retry", "event_id", event.ID, "max_retries", maxRetries)
	
	// Initial broadcast attempt
	results = BroadcastEvent(event, relays, pool)
	
	// Collect failed relays for retry
	for _, result := range results {
		if !result.Success {
			failedRelays = append(failedRelays, result.RelayURL)
		}
	}
	
	// Retry failed relays
	for attempt := 2; attempt <= maxRetries && len(failedRelays) > 0; attempt++ {
		log.ClientCore().Debug("Retry attempt", "attempt", attempt, "failed_relay_count", len(failedRelays))
		
		// Wait before retry
		time.Sleep(time.Duration(attempt) * time.Second)
		
		retryResults := BroadcastEvent(event, failedRelays, pool)
		
		// Update results and collect still-failed relays
		newFailedRelays := make([]string, 0)
		retryIndex := 0
		
		for i, result := range results {
			if !result.Success {
				// Update with retry result
				results[i] = retryResults[retryIndex]
				retryIndex++
				
				// If still failed, add to next retry list
				if !results[i].Success {
					newFailedRelays = append(newFailedRelays, result.RelayURL)
				}
			}
		}
		
		failedRelays = newFailedRelays
	}
	
	// Log final summary
	successful := 0
	for _, result := range results {
		if result.Success {
			successful++
		}
	}
	
	log.ClientCore().Info("Broadcast with retry completed", 
		"event_id", event.ID,
		"successful", successful,
		"total", len(relays),
		"attempts", maxRetries)
	
	return results
}

// PublishEvent is a high-level function to build, sign, and broadcast an event
func PublishEvent(client *Client, signer *EventSigner, eventBuilder *EventBuilder, targetRelays []string) (*nostr.Event, []BroadcastResult, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("client cannot be nil")
	}
	
	if signer == nil {
		return nil, nil, fmt.Errorf("signer cannot be nil")
	}
	
	if eventBuilder == nil {
		return nil, nil, fmt.Errorf("event builder cannot be nil")
	}
	
	// Build the event
	event := eventBuilder.Build()
	
	// Sign the event
	if err := signer.SignEvent(event); err != nil {
		return nil, nil, fmt.Errorf("failed to sign event: %w", err)
	}
	
	// Validate the event
	if err := ValidateEventStructure(event); err != nil {
		return nil, nil, fmt.Errorf("event validation failed: %w", err)
	}
	
	// Use provided relays or fall back to user's write relays
	relays := targetRelays
	if len(relays) == 0 {
		mailboxes, err := client.GetUserRelays(signer.GetPublicKey())
		if err == nil && mailboxes != nil {
			relays = mailboxes.Write
			if len(relays) == 0 {
				relays = mailboxes.Both
			}
		}
		
		// Final fallback to default relays
		if len(relays) == 0 {
			relays = client.config.DefaultRelays
		}
	}
	
	log.ClientCore().Info("Publishing event", "event_id", event.ID, "kind", event.Kind, "relay_count", len(relays))
	
	// Broadcast the event
	results := BroadcastEvent(event, relays, client.relayPool)
	
	return event, results, nil
}

// PublishEventWithRetry publishes an event with retry logic
func PublishEventWithRetry(client *Client, signer *EventSigner, eventBuilder *EventBuilder, targetRelays []string, maxRetries int) (*nostr.Event, []BroadcastResult, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("client cannot be nil")
	}
	
	if signer == nil {
		return nil, nil, fmt.Errorf("signer cannot be nil")
	}
	
	if eventBuilder == nil {
		return nil, nil, fmt.Errorf("event builder cannot be nil")
	}
	
	// Build the event
	event := eventBuilder.Build()
	
	// Sign the event
	if err := signer.SignEvent(event); err != nil {
		return nil, nil, fmt.Errorf("failed to sign event: %w", err)
	}
	
	// Validate the event
	if err := ValidateEventStructure(event); err != nil {
		return nil, nil, fmt.Errorf("event validation failed: %w", err)
	}
	
	// Use provided relays or fall back to user's write relays
	relays := targetRelays
	if len(relays) == 0 {
		mailboxes, err := client.GetUserRelays(signer.GetPublicKey())
		if err == nil && mailboxes != nil {
			relays = mailboxes.Write
			if len(relays) == 0 {
				relays = mailboxes.Both
			}
		}
		
		// Final fallback to default relays
		if len(relays) == 0 {
			relays = client.config.DefaultRelays
		}
	}
	
	log.ClientCore().Info("Publishing event with retry", "event_id", event.ID, "kind", event.Kind, "relay_count", len(relays))
	
	// Broadcast the event with retry
	results := BroadcastWithRetry(event, relays, client.relayPool, maxRetries)
	
	return event, results, nil
}

// BroadcastSummary provides a summary of broadcast results
type BroadcastSummary struct {
	TotalRelays    int
	Successful     int
	Failed         int
	SuccessRate    float64
	AverageDuration time.Duration
	Errors         []string
}

// SummarizeBroadcast creates a summary of broadcast results
func SummarizeBroadcast(results []BroadcastResult) BroadcastSummary {
	summary := BroadcastSummary{
		TotalRelays: len(results),
		Errors:      make([]string, 0),
	}
	
	var totalDuration time.Duration
	
	for _, result := range results {
		if result.Success {
			summary.Successful++
		} else {
			summary.Failed++
			if result.Error != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %v", result.RelayURL, result.Error))
			}
		}
		totalDuration += result.Duration
	}
	
	if summary.TotalRelays > 0 {
		summary.SuccessRate = float64(summary.Successful) / float64(summary.TotalRelays) * 100
		summary.AverageDuration = totalDuration / time.Duration(summary.TotalRelays)
	}
	
	return summary
}