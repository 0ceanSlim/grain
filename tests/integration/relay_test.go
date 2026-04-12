package integration

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

func TestMain(m *testing.M) {
	// Wait for every per-scenario relay to be ready before running tests.
	// We can't use *testing.T here (TestMain has no real T), so surface
	// readiness failures via log.Fatalf before m.Run().
	if err := tests.WaitForAllRelaysReady(120); err != nil {
		log.Fatalf("relay readiness check failed: %v", err)
	}
	os.Exit(m.Run())
}

func TestBasicConnection(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()
	t.Log("Successfully connected to relay")
}

func TestPublishAndQuery(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish a kind-1 text note
	evt := kp.SignEvent(1, "integration test note", nil)
	client.SendEvent(evt)

	accepted, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if !accepted {
		t.Fatalf("Event rejected: %s", reason)
	}
	t.Logf("Event %s accepted", evt.ID[:8])

	// Query it back
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"ids": []string{evt.ID},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0]["id"] != evt.ID {
		t.Fatalf("Returned event ID mismatch: %v != %s", events[0]["id"], evt.ID)
	}
	t.Logf("Successfully queried event back by ID")
}

func TestQueryByAuthor(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish two events from same author
	evt1 := kp.SignEvent(1, "author test note 1", nil)
	evt2 := kp.SignEvent(1, "author test note 2", nil)
	client.SendEvent(evt1)
	client.ExpectOK(evt1.ID, 5*time.Second)
	client.SendEvent(evt2)
	client.ExpectOK(evt2.ID, 5*time.Second)

	// Query by author
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) < 2 {
		t.Fatalf("Expected at least 2 events from author, got %d", len(events))
	}
	t.Logf("Found %d events from author", len(events))
}

func TestQueryByKind(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	// Publish different kinds
	evt1 := kp.SignEvent(1, "kind 1 note", nil)
	client.SendEvent(evt1)
	client.ExpectOK(evt1.ID, 5*time.Second)

	// Query specifically for kind 1 from this author
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) == 0 {
		t.Fatalf("Expected at least 1 kind-1 event")
	}

	for _, e := range events {
		if kind, ok := e["kind"].(float64); ok && int(kind) != 1 {
			t.Fatalf("Got event with wrong kind: %v", kind)
		}
	}
	t.Logf("Kind filter working correctly, got %d events", len(events))
}

func TestSubscriptionEOSE(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	// Subscribe to events (even if none exist, should get EOSE)
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"kinds": []int{99999}, // unlikely to have any
		"limit": 1,
	})

	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) != 0 {
		t.Logf("Got %d events for rare kind (unexpected but not fatal)", len(events))
	}
	t.Log("EOSE received for empty subscription")
}

func TestCloseSubscription(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"kinds": []int{1},
		"limit": 1,
	})

	// Read until EOSE
	client.ExpectEOSE(subID, 5*time.Second)

	// Close subscription
	client.SendMessage([]interface{}{"CLOSE", subID})
	t.Log("Subscription closed successfully")
}
