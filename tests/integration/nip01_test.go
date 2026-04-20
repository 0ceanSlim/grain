package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// NIP-01 compliance tests.
//
// These verify core Nostr protocol behavior as specified in NIP-01:
// https://github.com/nostr-protocol/nips/blob/master/01.md
//
// All tests run against the default relay (port 8182) which has permissive
// rate limits (500/s) and no blacklist/whitelist restrictions.

// TestNIP01_EventOKFormat verifies the OK message format:
// ["OK", <event-id>, <true|false>, <message>]
func TestNIP01_EventOKFormat(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	evt := kp.SignEvent(1, "nip01-ok-format", nil)
	client.SendEvent(evt)

	// Read raw to verify exact structure
	raw := client.ReadMessageRaw(5 * time.Second)
	var msg []json.RawMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("Failed to parse OK message: %v", err)
	}

	if len(msg) != 4 {
		t.Fatalf("OK message must have exactly 4 elements, got %d: %s", len(msg), string(raw))
	}

	var verb string
	json.Unmarshal(msg[0], &verb)
	if verb != "OK" {
		t.Fatalf("Expected OK verb, got %q", verb)
	}

	var id string
	json.Unmarshal(msg[1], &id)
	if id != evt.ID {
		t.Fatalf("OK event ID mismatch: %s != %s", id, evt.ID)
	}

	var accepted bool
	if err := json.Unmarshal(msg[2], &accepted); err != nil {
		t.Fatalf("OK third element must be boolean: %v", err)
	}
	if !accepted {
		var reason string
		json.Unmarshal(msg[3], &reason)
		t.Fatalf("Event should be accepted, got rejected: %s", reason)
	}
}

// TestNIP01_TagsNeverNull verifies that events with no tags serialize with
// "tags":[] (empty array) instead of "tags":null. NIP-01 specifies tags as
// a JSON array, and null breaks clients.
func TestNIP01_TagsNeverNull(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish event with no tags
	evt := kp.SignEvent(1, "nip01-tags-null-check", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Event rejected: %s", reason)
	}

	// Query it back and check raw JSON
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"ids": []string{evt.ID},
	})

	// Read raw messages until we find the EVENT for our sub
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := client.ReadMessageRaw(time.Until(deadline))
		rawStr := string(raw)

		// Check if this is an EVENT message for our subscription
		if strings.Contains(rawStr, `"EVENT"`) || strings.Contains(rawStr, `"EOSE"`) {
			var parsed []json.RawMessage
			json.Unmarshal(raw, &parsed)
			if len(parsed) >= 2 {
				var verb string
				json.Unmarshal(parsed[0], &verb)

				if verb == "EVENT" && len(parsed) >= 3 {
					// Check the event payload for null tags
					evtJSON := string(parsed[2])
					if strings.Contains(evtJSON, `"tags":null`) {
						t.Fatal("tags must be [] (empty array), not null")
					}
					if !strings.Contains(evtJSON, `"tags":[]`) &&
						!strings.Contains(evtJSON, `"tags": []`) {
						// tags might have whitespace variations, so parse and check
						var evtMap map[string]interface{}
						json.Unmarshal(parsed[2], &evtMap)
						tagsRaw, exists := evtMap["tags"]
						if !exists {
							t.Fatal("tags field missing from event")
						}
						if tagsRaw == nil {
							t.Fatal("tags must be [] (empty array), not null")
						}
						arr, ok := tagsRaw.([]interface{})
						if !ok {
							t.Fatalf("tags must be an array, got %T", tagsRaw)
						}
						if len(arr) != 0 {
							t.Fatalf("expected empty tags array, got %d elements", len(arr))
						}
					}
					return // pass
				}

				if verb == "EOSE" {
					t.Fatal("Got EOSE without receiving the event")
				}
			}
		}
	}
	t.Fatal("Timed out waiting for EVENT response")
}

// TestNIP01_TagFilterE verifies that #e tag filters work in subscriptions.
// NIP-01: a filter with "#e": ["<id>"] should return events that have an
// ["e", "<id>"] tag.
func TestNIP01_TagFilterE(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish a reference event
	refEvt := kp.SignEvent(1, "nip01-tag-filter-ref", nil)
	client.SendEvent(refEvt)
	ok, reason := client.ExpectOK(refEvt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Reference event rejected: %s", reason)
	}

	// Publish an event that tags the reference
	taggedEvt := kp.SignEvent(1, "nip01-tag-filter-reply", [][]string{
		{"e", refEvt.ID},
	})
	client.SendEvent(taggedEvt)
	ok, reason = client.ExpectOK(taggedEvt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Tagged event rejected: %s", reason)
	}

	// Publish an unrelated event (no e tag)
	unrelated := kp.SignEvent(1, "nip01-tag-filter-unrelated", nil)
	client.SendEvent(unrelated)
	client.ExpectOK(unrelated.ID, 5*time.Second)

	// Query with #e filter for the reference event ID
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"#e":      []string{refEvt.ID},
		"authors": []string{kp.PubKey},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) == 0 {
		t.Fatal("Expected at least 1 event matching #e filter, got 0")
	}

	// The tagged event should be in results
	found := false
	for _, e := range events {
		if e["id"] == taggedEvt.ID {
			found = true
		}
		// The unrelated event should NOT be in results
		if e["id"] == unrelated.ID {
			t.Fatal("Unrelated event (no e tag) should not match #e filter")
		}
	}
	if !found {
		t.Fatalf("Tagged event %s not found in #e filter results", taggedEvt.ID[:8])
	}
}

// TestNIP01_TagFilterP verifies that #p tag filters work.
func TestNIP01_TagFilterP(t *testing.T) {
	kp1 := tests.NewTestKeypair()
	kp2 := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish an event from kp1 that mentions kp2
	mentionEvt := kp1.SignEvent(1, "nip01-p-tag-mention", [][]string{
		{"p", kp2.PubKey},
	})
	client.SendEvent(mentionEvt)
	ok, reason := client.ExpectOK(mentionEvt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Mention event rejected: %s", reason)
	}

	// Query with #p filter for kp2's pubkey
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"#p":      []string{kp2.PubKey},
		"authors": []string{kp1.PubKey},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) == 0 {
		t.Fatal("Expected at least 1 event matching #p filter, got 0")
	}

	found := false
	for _, e := range events {
		if e["id"] == mentionEvt.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("Mention event %s not found in #p filter results", mentionEvt.ID[:8])
	}
}

// TestNIP01_TagFilterCustom verifies that custom single-letter tag filters work.
// NIP-01 says any single-letter key in a filter prefixed with # is a tag filter.
func TestNIP01_TagFilterCustom(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish event with custom "t" (topic) tag
	taggedEvt := kp.SignEvent(1, "nip01-custom-tag", [][]string{
		{"t", "nostr"},
		{"t", "grain"},
	})
	client.SendEvent(taggedEvt)
	ok, reason := client.ExpectOK(taggedEvt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Event rejected: %s", reason)
	}

	// Query with #t filter
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"#t":      []string{"nostr"},
		"authors": []string{kp.PubKey},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) == 0 {
		t.Fatal("Expected at least 1 event matching #t filter, got 0")
	}

	found := false
	for _, e := range events {
		if e["id"] == taggedEvt.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("Tagged event %s not found in #t filter results", taggedEvt.ID[:8])
	}
}

// TestNIP01_FilterLimit verifies that the limit field caps the number of
// returned events.
func TestNIP01_FilterLimit(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish 5 events (unique content so each gets a distinct ID)
	for i := 0; i < 5; i++ {
		evt := kp.SignEvent(1, fmt.Sprintf("nip01-limit-%d", i), nil)
		client.SendEvent(evt)
		ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
		if !ok {
			t.Fatalf("Event %d rejected: %s", i, reason)
		}
	}

	// Query with limit 2
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
		"limit":   2,
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) > 2 {
		t.Fatalf("Limit 2 should return at most 2 events, got %d", len(events))
	}
	if len(events) == 0 {
		t.Fatal("Expected at least 1 event, got 0")
	}
}

// TestNIP01_DuplicateEvent verifies that sending the same event twice
// is handled properly. NIP-01 says relays MAY send OK false for duplicates.
func TestNIP01_DuplicateEvent(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	evt := kp.SignEvent(1, "nip01-duplicate-test", nil)

	// First send — should be accepted
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("First send should be accepted, got rejected: %s", reason)
	}

	// Second send — same event. Relay should respond with OK.
	// NIP-01 allows OK true (idempotent) or OK false with "duplicate:" prefix.
	client.SendEvent(evt)
	ok2, reason2 := client.ExpectOK(evt.ID, 5*time.Second)

	if ok2 {
		// Accepted idempotently — that's fine
		return
	}

	// Rejected — reason should indicate duplicate, not an error
	if !tests.ContainsAny(reason2, "duplicate", "blocked", "already") {
		t.Fatalf("Duplicate rejection should mention duplicate/blocked, got: %q", reason2)
	}
}

// TestNIP01_InvalidSignature verifies that events with tampered signatures
// are rejected with an OK false and "invalid:" prefixed message.
func TestNIP01_InvalidSignature(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	evt := kp.SignEvent(1, "nip01-bad-sig", nil)

	// Tamper with the signature — flip first byte
	sigBytes := []byte(evt.Sig)
	if sigBytes[0] == 'a' {
		sigBytes[0] = 'b'
	} else {
		sigBytes[0] = 'a'
	}
	evt.Sig = string(sigBytes)

	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if ok {
		t.Fatal("Event with invalid signature should be rejected")
	}
	if !strings.HasPrefix(reason, "invalid:") {
		t.Fatalf("Rejection reason should start with 'invalid:', got %q", reason)
	}
}

// TestNIP01_MultipleFiltersOR verifies that multiple filter objects in a
// single REQ use OR logic: events matching ANY filter are returned.
func TestNIP01_MultipleFiltersOR(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish a kind-1 and a kind-30023 event
	evt1 := kp.SignEvent(1, "nip01-multi-filter-kind1", nil)
	client.SendEvent(evt1)
	ok, reason := client.ExpectOK(evt1.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Kind 1 event rejected: %s", reason)
	}

	evt30023 := kp.SignEvent(30023, "nip01-multi-filter-kind30023", [][]string{
		{"d", "test-article"},
	})
	client.SendEvent(evt30023)
	ok, reason = client.ExpectOK(evt30023.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Kind 30023 event rejected: %s", reason)
	}

	// Subscribe with two filters (OR logic)
	subID := tests.RandomSubID()
	client.Subscribe(subID,
		map[string]interface{}{
			"authors": []string{kp.PubKey},
			"kinds":   []int{1},
			"ids":     []string{evt1.ID},
		},
		map[string]interface{}{
			"authors": []string{kp.PubKey},
			"kinds":   []int{30023},
			"ids":     []string{evt30023.ID},
		},
	)

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) < 2 {
		t.Fatalf("Expected at least 2 events from OR filter, got %d", len(events))
	}

	foundKind1 := false
	foundKind30023 := false
	for _, e := range events {
		if e["id"] == evt1.ID {
			foundKind1 = true
		}
		if e["id"] == evt30023.ID {
			foundKind30023 = true
		}
	}
	if !foundKind1 {
		t.Fatal("Kind 1 event not returned by OR filter")
	}
	if !foundKind30023 {
		t.Fatal("Kind 30023 event not returned by OR filter")
	}
}

// TestNIP01_ReplaceSubscription verifies that sending a new REQ with the
// same subscription ID replaces the old subscription.
func TestNIP01_ReplaceSubscription(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	subID := "nip01-replace-sub"

	// First subscription: kind 99998 (nothing should match)
	client.Subscribe(subID, map[string]interface{}{
		"kinds": []int{99998},
		"limit": 1,
	})
	events1 := client.ExpectEOSE(subID, 5*time.Second)
	if len(events1) != 0 {
		t.Fatalf("Expected 0 events for kind 99998, got %d", len(events1))
	}

	// Publish a kind-1 event
	evt := kp.SignEvent(1, "nip01-replace-sub-test", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Event rejected: %s", reason)
	}

	// Replace subscription with same subID, now filtering for kind 1
	client.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
		"ids":     []string{evt.ID},
	})

	events2 := client.ExpectEOSE(subID, 5*time.Second)
	if len(events2) == 0 {
		t.Fatal("Replaced subscription should return events matching new filter")
	}

	found := false
	for _, e := range events2 {
		if e["id"] == evt.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("Replaced subscription did not return the expected event")
	}
}

// TestNIP01_CloseStopsDelivery verifies that CLOSE stops live event delivery
// on a subscription.
func TestNIP01_CloseStopsDelivery(t *testing.T) {
	kp := tests.NewTestKeypair()

	subscriber := tests.NewTestClient(t)
	defer subscriber.Close()

	publisher := tests.NewTestClient(t)
	defer publisher.Close()

	subID := tests.RandomSubID()
	subscriber.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
	})
	subscriber.ExpectEOSE(subID, 5*time.Second)

	// Close the subscription
	subscriber.SendMessage([]interface{}{"CLOSE", subID})

	// Small delay for the server to process the CLOSE
	time.Sleep(200 * time.Millisecond)

	// Publish an event — subscriber should NOT receive it
	evt := kp.SignEvent(1, "nip01-close-stops-delivery", nil)
	publisher.SendEvent(evt)
	publisher.ExpectOK(evt.ID, 5*time.Second)

	// Try to read — should timeout (no delivery after CLOSE)
	msg, err := subscriber.TryReadMessage(2 * time.Second)
	if err == nil && msg != nil {
		if len(msg) >= 3 && msg[0] == "EVENT" {
			t.Fatal("Should not receive EVENT after CLOSE")
		}
		// Other messages (like NOTICE) are acceptable
	}
	// Timeout (err != nil) means no delivery — correct behavior
}

// TestNIP01_LiveDeliveryWithTagFilter verifies that live subscription
// delivery respects tag filters.
func TestNIP01_LiveDeliveryWithTagFilter(t *testing.T) {
	kp := tests.NewTestKeypair()

	subscriber := tests.NewTestClient(t)
	defer subscriber.Close()

	publisher := tests.NewTestClient(t)
	defer publisher.Close()

	targetID := tests.NewTestKeypair().PubKey // random ID to use as e-tag value

	// Subscribe for events with a specific #e tag
	subID := tests.RandomSubID()
	subscriber.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
		"#e":      []string{targetID},
	})
	subscriber.ExpectEOSE(subID, 5*time.Second)

	// Publish an event WITHOUT the e tag — should NOT be delivered
	noTag := kp.SignEvent(1, "nip01-no-matching-tag", nil)
	publisher.SendEvent(noTag)
	publisher.ExpectOK(noTag.ID, 5*time.Second)

	// Publish an event WITH the matching e tag — should be delivered
	withTag := kp.SignEvent(1, "nip01-matching-tag", [][]string{
		{"e", targetID},
	})
	publisher.SendEvent(withTag)
	publisher.ExpectOK(withTag.ID, 5*time.Second)

	// Read from subscriber — should get the tagged event
	msg := subscriber.ReadMessage(5 * time.Second)
	if len(msg) < 3 || msg[0] != "EVENT" {
		t.Fatalf("Expected EVENT message, got: %v", msg)
	}
	evtMap, ok := msg[2].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected event map, got %T", msg[2])
	}
	if evtMap["id"] != withTag.ID {
		t.Fatalf("Expected tagged event %s, got %v", withTag.ID[:8], evtMap["id"])
	}

	// Verify the non-matching event was NOT delivered
	extra, err := subscriber.TryReadMessage(2 * time.Second)
	if err == nil && extra != nil {
		if len(extra) >= 3 && extra[0] == "EVENT" {
			if eMap, ok := extra[2].(map[string]interface{}); ok {
				if eMap["id"] == noTag.ID {
					t.Fatal("Event without matching tag should not be delivered")
				}
			}
		}
	}
}

// TestNIP01_EOSEAlwaysSent verifies that EOSE is always sent even for
// subscriptions with zero historical results.
func TestNIP01_EOSEAlwaysSent(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	// Subscribe for a kind that almost certainly has no events
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"kinds":   []int{59999},
		"authors": []string{tests.NewTestKeypair().PubKey},
		"limit":   1,
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) != 0 {
		t.Logf("Unexpected events for kind 59999 (got %d), but EOSE was sent correctly", len(events))
	}
}

// TestNIP01_EventIDIntegrity verifies that the event returned by the relay
// has the same id, pubkey, kind, and content as what was published.
func TestNIP01_EventIDIntegrity(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	evt := kp.SignEvent(1, "nip01-integrity-check-content", [][]string{
		{"t", "integrity"},
	})
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if !ok {
		t.Fatalf("Event rejected: %s", reason)
	}

	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"ids": []string{evt.ID},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	e := events[0]

	if e["id"] != evt.ID {
		t.Fatalf("ID mismatch: %v != %s", e["id"], evt.ID)
	}
	if e["pubkey"] != evt.PubKey {
		t.Fatalf("PubKey mismatch: %v != %s", e["pubkey"], evt.PubKey)
	}
	if int(e["kind"].(float64)) != evt.Kind {
		t.Fatalf("Kind mismatch: %v != %d", e["kind"], evt.Kind)
	}
	if e["content"] != evt.Content {
		t.Fatalf("Content mismatch: %v != %s", e["content"], evt.Content)
	}

	// Verify tags round-trip as a proper array (not null)
	tags, ok := e["tags"].([]interface{})
	if !ok {
		t.Fatalf("Tags should be array, got %T", e["tags"])
	}
	if len(tags) < 1 {
		t.Fatal("Expected at least 1 tag")
	}
	tag0, ok := tags[0].([]interface{})
	if !ok || len(tag0) < 1 {
		t.Fatalf("Expected tag array with at least 1 element, got %v", tags[0])
	}
	if tag0[0] != "t" {
		t.Fatalf("Tag key mismatch: expected 't', got %v", tag0[0])
	}
}
