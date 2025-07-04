package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

const (
	TestRelayURL = "ws://localhost:8182"
	TestHTTPURL  = "http://localhost:8182"
)

// TestClient wraps a WebSocket connection for testing
type TestClient struct {
	conn *websocket.Conn
	t    *testing.T
}

// NewTestClient creates a new WebSocket connection to the test relay
func NewTestClient(t *testing.T) *TestClient {
	origin := "http://localhost/"
	conn, err := websocket.Dial(TestRelayURL, "", origin)
	if err != nil {
		t.Fatalf("Failed to connect to test relay: %v", err)
	}

	return &TestClient{
		conn: conn,
		t:    t,
	}
}

// SendMessage sends a message to the relay
func (c *TestClient) SendMessage(msg interface{}) {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatalf("Failed to marshal message: %v", err)
	}

	_, err = c.conn.Write(msgBytes)
	if err != nil {
		c.t.Fatalf("Failed to send message: %v", err)
	}
}

// ReadMessage reads a message from the relay with timeout
func (c *TestClient) ReadMessage(timeout time.Duration) []interface{} {
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	msgBytes := make([]byte, 4096)
	n, err := c.conn.Read(msgBytes)
	if err != nil {
		c.t.Fatalf("Failed to read message: %v", err)
	}

	var msg []interface{}
	err = json.Unmarshal(msgBytes[:n], &msg)
	if err != nil {
		c.t.Fatalf("Failed to unmarshal message: %v", err)
	}

	return msg
}

// Close closes the WebSocket connection
func (c *TestClient) Close() {
	c.conn.Close()
}

// WaitForRelayReady waits for the relay to be ready
func WaitForRelayReady(t *testing.T, maxAttempts int) {
	for i := 0; i < maxAttempts; i++ {
		resp, err := http.Get(TestHTTPURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Relay not ready after %d attempts", maxAttempts)
}
