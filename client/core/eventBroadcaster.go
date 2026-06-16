package core

import (
	"fmt"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// BroadcastResult represents the result of broadcasting to a single relay.
// Success means the EVENT was sent; Accepted is the relay's NIP-20 OK verdict
// (whether it actually stored the event), with Reason carrying any message.
type BroadcastResult struct {
	RelayURL string
	Success  bool
	Accepted bool
	Reason   string
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

	// Start collecting NIP-20 OK responses BEFORE sending, so a relay that
	// replies fast can't beat the waiter into place.
	okCh := pool.messageRouter.RegisterOKWaiter(event.ID, len(relays))
	defer pool.messageRouter.UnregisterOKWaiter(event.ID)

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

	// Collect OK responses for the relays we successfully sent to, matched by
	// relay URL, within a short window. A relay that never answers is left as
	// Accepted=false (sent but unconfirmed).
	collectOKResponses(okCh, results)

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

// collectOKResponses waits on okCh for NIP-20 OK responses and fills the
// Accepted/Reason fields of the matching results, for up to a short window. It
// only waits for relays we successfully sent to; a relay that never answers is
// left Accepted=false (sent but unconfirmed).
func collectOKResponses(okCh <-chan OKResult, results []BroadcastResult) {
	relayIndex := make(map[string]int, len(results))
	pending := 0
	for i := range results {
		relayIndex[results[i].RelayURL] = i
		if results[i].Success {
			pending++
		}
	}
	if pending == 0 {
		return
	}

	deadline := time.After(5 * time.Second)
	received := make(map[string]bool, pending)
	for pending > 0 {
		select {
		case ok := <-okCh:
			if received[ok.Relay] {
				continue
			}
			received[ok.Relay] = true
			if idx, found := relayIndex[ok.Relay]; found {
				results[idx].Accepted = ok.Accepted
				results[idx].Reason = ok.Reason
			}
			pending--
		case <-deadline:
			return
		}
	}
}

// broadcastEventStream broadcasts like BroadcastEvent but emits each relay's
// result on the returned channel the moment it resolves: a send failure right
// away, an accept/reject when that relay's NIP-20 OK arrives, and a "sent but no
// response" at the collect deadline. The channel is closed once every relay has
// resolved. This powers the live, count-up publish toast.
func broadcastEventStream(event *nostr.Event, relays []string, pool *RelayPool) <-chan BroadcastResult {
	out := make(chan BroadcastResult, len(relays)) // buffered to the relay count: each relay emits exactly once, so sends never block

	go func() {
		defer close(out)
		if event == nil || len(relays) == 0 {
			return
		}

		// Register the OK waiter BEFORE sending so a fast relay can't beat it.
		okCh := pool.messageRouter.RegisterOKWaiter(event.ID, len(relays))
		defer pool.messageRouter.UnregisterOKWaiter(event.ID)

		eventMessage := []interface{}{"EVENT", event}

		sent := make([]BroadcastResult, len(relays))
		relayIndex := make(map[string]int, len(relays))
		for i, u := range relays {
			relayIndex[u] = i
		}

		// Send to every relay concurrently. A send FAILURE is final — emit it
		// immediately; a successful send waits for the relay's OK below.
		var wg sync.WaitGroup
		var mu sync.Mutex
		for i, relayURL := range relays {
			wg.Add(1)
			go func(index int, relay string) {
				defer wg.Done()
				start := time.Now()
				r := broadcastToSingleRelay(relay, eventMessage, pool)
				r.RelayURL = relay
				r.Duration = time.Since(start)
				mu.Lock()
				sent[index] = r
				mu.Unlock()
				if !r.Success {
					out <- r
				}
			}(i, relayURL)
		}
		wg.Wait()

		pending := 0
		for i := range sent {
			if sent[i].Success {
				pending++
			}
		}
		if pending == 0 {
			return
		}

		deadline := time.After(5 * time.Second)
		received := make(map[string]bool, pending)
		for pending > 0 {
			select {
			case ok := <-okCh:
				if received[ok.Relay] {
					continue
				}
				idx, found := relayIndex[ok.Relay]
				if !found || !sent[idx].Success {
					continue
				}
				received[ok.Relay] = true
				sent[idx].Accepted = ok.Accepted
				sent[idx].Reason = ok.Reason
				out <- sent[idx]
				pending--
			case <-deadline:
				// Emit the relays that were sent but never answered.
				for i := range sent {
					if sent[i].Success && !received[sent[i].RelayURL] {
						out <- sent[i] // Accepted=false, no reason => "no response"
					}
				}
				return
			}
		}
	}()

	return out
}

// PublishEventStream broadcasts an already-signed event to the given relays and
// returns a channel that emits each relay's result as it resolves. The caller
// ranges the channel until it closes. Used by the streaming publish endpoint to
// drive the live broadcast toast.
func (c *Client) PublishEventStream(event *nostr.Event, relays []string) <-chan BroadcastResult {
	return broadcastEventStream(event, relays, c.relayPool)
}

// broadcastToSingleRelay broadcasts to a single relay
// publishConnectTimeout bounds the redial of a dropped publish target before the
// broadcast gives up on it with a server-timeout result.
const publishConnectTimeout = 10 * time.Second

func broadcastToSingleRelay(relayURL string, message []interface{}, pool *RelayPool) BroadcastResult {
	// Make sure the target relay is connected before sending. A publish target
	// that has dropped would otherwise fail instantly with "not connected";
	// redial it (an explicit publish ignores dial backoff), bounded by
	// publishConnectTimeout, and report a server timeout if it won't come up.
	if err := pool.EnsureConnectedForSend(relayURL, publishConnectTimeout); err != nil {
		log.ClientCore().Warn("Couldn't connect to relay to broadcast", "relay", relayURL, "error", err)
		return BroadcastResult{
			Success: false,
			Error:   err,
			Message: "server timeout (couldn't connect)",
		}
	}

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
		log.ClientCore().Warn("Failed to get user relays, using index relays", "pubkey", pubkey, "error", err)
		return BroadcastEvent(event, client.config.IndexRelays, client.relayPool)
	}

	// Use write relays for broadcasting
	relays := mailboxes.Write
	if len(relays) == 0 {
		// Fall back to 'both' relays if no write-specific relays
		relays = mailboxes.Both
	}

	if len(relays) == 0 {
		// Fall back to index relays if user has no relay preferences
		log.ClientCore().Warn("User has no relay preferences, using index relays", "pubkey", pubkey)
		relays = client.config.IndexRelays
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

	// Use provided relays, or route by the outbox model: the author's outbox
	// plus every p-tagged recipient's inbox (so replies / mentions reach them).
	relays := targetRelays
	if len(relays) == 0 {
		relays = client.RoutePublish(event)
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

	// Use provided relays, or route by the outbox model: the author's outbox
	// plus every p-tagged recipient's inbox (so replies / mentions reach them).
	relays := targetRelays
	if len(relays) == 0 {
		relays = client.RoutePublish(event)
	}

	log.ClientCore().Info("Publishing event with retry", "event_id", event.ID, "kind", event.Kind, "relay_count", len(relays))

	// Broadcast the event with retry
	results := BroadcastWithRetry(event, relays, client.relayPool, maxRetries)

	return event, results, nil
}

// BroadcastSummary provides a summary of broadcast results
type BroadcastSummary struct {
	TotalRelays     int
	Successful      int
	Failed          int
	SuccessRate     float64
	AverageDuration time.Duration
	Errors          []string
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
