package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

func TestMain(m *testing.M) {
	// Wait for relay to be ready before running tests
	tests.WaitForRelayReady(&testing.T{}, 30)
	m.Run()
}

func TestBasicConnection(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	t.Log("✅ Successfully connected to relay")
}

func TestEventPublishing(t *testing.T) {
	// TODO: Implement proper Nostr event signing and publishing test
	// This is a placeholder that passes until we implement real event creation with proper signatures
	t.Logf("⚠️ Event publishing test skipped - placeholder implementation")
	t.Skip("Event publishing test not yet implemented with proper Nostr event signing")
}

func TestEventSubscription(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()

	// Subscribe to events
	filter := map[string]interface{}{
		"kinds": []int{1},
		"limit": 1,
	}

	client.SendMessage([]interface{}{"REQ", "test-sub", filter})

	// Should receive EOSE
	for {
		response := client.ReadMessage(5 * time.Second)

		if len(response) < 2 {
			continue
		}

		if response[0] == "EOSE" && response[1] == "test-sub" {
			t.Log("✅ Received EOSE - subscription established")
			break
		}

		if response[0] == "EVENT" {
			t.Logf("📦 Received event: %s", response[1])
		}
	}

	// Close subscription
	client.SendMessage([]interface{}{"CLOSE", "test-sub"})
	t.Log("✅ Subscription test completed")
}
