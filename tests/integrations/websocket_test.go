package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

func TestWebSocketTimeout(t *testing.T) {
	client := tests.NewTestClient(t)
	defer client.Close()
	
	// Send invalid message and expect connection to handle it gracefully
	client.SendMessage([]interface{}{"INVALID_MESSAGE_TYPE"})
	
	// Should still be able to send valid messages
	client.SendMessage([]interface{}{"REQ", "test", map[string]interface{}{"kinds": []int{1}}})
	
	response := client.ReadMessage(5 * time.Second)
	if response[0] == "EOSE" {
		t.Log("✅ Relay handled invalid message gracefully")
	}
}

func TestMultipleClients(t *testing.T) {
	// Test multiple concurrent connections
	clients := make([]*tests.TestClient, 3)
	
	for i := 0; i < 3; i++ {
		clients[i] = tests.NewTestClient(t)
		defer clients[i].Close()
	}

	// Add delay between subscriptions
    for i, client := range clients {
        subID := fmt.Sprintf("multi-test-%d", i)
        client.SendMessage([]interface{}{"REQ", subID, map[string]interface{}{"kinds": []int{1}}})
        time.Sleep(50 * time.Millisecond) // Prevent rate limiting
    }
	
	// All clients subscribe
	for i, client := range clients {
		subID := fmt.Sprintf("multi-test-%d", i)
		client.SendMessage([]interface{}{"REQ", subID, map[string]interface{}{"kinds": []int{1}}})
	}
	
	for i, client := range clients {
		response := client.ReadMessage(5 * time.Second)
		if len(response) > 0 {
			switch response[0] {
			case "CLOSED":
				t.Errorf("❌ Client %d subscription was closed: %v", i, response)
			case "EOSE":
				t.Logf("✅ Client %d subscription successful", i)
			default:
				t.Logf("⚠️ Client %d received unexpected response: %v", i, response[0])
			}
		}
	}
}