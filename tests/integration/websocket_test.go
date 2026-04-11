package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

func TestInvalidMessage(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	// Send invalid message type
	client.SendMessage([]interface{}{"INVALID_MESSAGE_TYPE"})

	// Should still be able to send valid messages after
	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{"kinds": []int{1}, "limit": 1})
	client.ExpectEOSE(subID, 5*time.Second)
	t.Log("Relay handled invalid message gracefully")
}

func TestMultipleClients(t *testing.T) {
	clients := make([]*tests.TestClient, 3)
	for i := 0; i < 3; i++ {
		clients[i] = tests.NewTestClient(t)
		defer clients[i].Close()
	}

	// Each client subscribes
	for i, client := range clients {
		subID := fmt.Sprintf("multi-%d", i)
		client.Subscribe(subID, map[string]interface{}{"kinds": []int{1}, "limit": 1})
		time.Sleep(50 * time.Millisecond) // avoid rate limiting
	}

	// All should get EOSE
	for i, client := range clients {
		subID := fmt.Sprintf("multi-%d", i)
		client.ExpectEOSE(subID, 5*time.Second)
		t.Logf("Client %d subscription successful", i)
	}
}

func TestLiveSubscription(t *testing.T) {
	kp := tests.NewTestKeypair()

	// Client 1 subscribes to this author's events
	subscriber := tests.NewTestClient(t)
	defer subscriber.Close()

	subID := tests.RandomSubID()
	subscriber.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{1},
	})
	subscriber.ExpectEOSE(subID, 5*time.Second)

	// Client 2 publishes an event
	publisher := tests.NewTestClient(t)
	defer publisher.Close()

	evt := kp.SignEvent(1, "live subscription test", nil)
	publisher.SendEvent(evt)
	accepted, reason := publisher.ExpectOK(evt.ID, 5*time.Second)
	if !accepted {
		t.Fatalf("Event rejected: %s", reason)
	}

	// Subscriber should receive the event in real-time
	msg := subscriber.ReadMessage(5 * time.Second)
	if len(msg) < 3 {
		t.Fatalf("Expected EVENT message, got: %v", msg)
	}
	if msg[0] != "EVENT" {
		t.Fatalf("Expected EVENT, got %v", msg[0])
	}
	if evtMap, ok := msg[2].(map[string]interface{}); ok {
		if evtMap["id"] != evt.ID {
			t.Fatalf("Received wrong event: %v != %s", evtMap["id"], evt.ID)
		}
	}
	t.Log("Live subscription received event in real-time")
}
