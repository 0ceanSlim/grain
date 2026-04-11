package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"golang.org/x/net/websocket"
)

const (
	TestRelayURL = "ws://localhost:8182"
	TestHTTPURL  = "http://localhost:8182"
)

// TestKeypair holds a private key and its corresponding public key hex for testing.
type TestKeypair struct {
	PrivKey *btcec.PrivateKey
	PubKey  string // hex-encoded x-only public key
}

// NewTestKeypair generates a random keypair for testing.
func NewTestKeypair() *TestKeypair {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		panic(fmt.Sprintf("failed to generate key: %v", err))
	}
	pubKey := privKey.PubKey()
	pubKeyHex := hex.EncodeToString(schnorr.SerializePubKey(pubKey))
	return &TestKeypair{PrivKey: privKey, PubKey: pubKeyHex}
}

// SignEvent creates a properly signed Nostr event.
func (kp *TestKeypair) SignEvent(kind int, content string, tags [][]string) nostr.Event {
	if tags == nil {
		tags = [][]string{}
	}
	now := time.Now().Unix()

	evt := nostr.Event{
		PubKey:    kp.PubKey,
		CreatedAt: now,
		Kind:      kind,
		Tags:      tags,
		Content:   content,
	}

	// Serialize per NIP-01: [0, pubkey, created_at, kind, tags, content]
	serialized, _ := json.Marshal([]interface{}{
		0, evt.PubKey, evt.CreatedAt, evt.Kind, evt.Tags, evt.Content,
	})

	// Compute ID
	hash := sha256.Sum256(serialized)
	evt.ID = hex.EncodeToString(hash[:])

	// Sign with schnorr
	sig, err := schnorr.Sign(kp.PrivKey, hash[:])
	if err != nil {
		panic(fmt.Sprintf("failed to sign event: %v", err))
	}
	evt.Sig = hex.EncodeToString(sig.Serialize())

	return evt
}

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

// SendEvent publishes an EVENT message to the relay
func (c *TestClient) SendEvent(evt nostr.Event) {
	c.SendMessage([]interface{}{"EVENT", evt})
}

// Subscribe sends a REQ with the given subscription ID and filters
func (c *TestClient) Subscribe(subID string, filters ...map[string]interface{}) {
	msg := make([]interface{}, 2+len(filters))
	msg[0] = "REQ"
	msg[1] = subID
	for i, f := range filters {
		msg[2+i] = f
	}
	c.SendMessage(msg)
}

// ReadMessage reads a message from the relay with timeout
func (c *TestClient) ReadMessage(timeout time.Duration) []interface{} {
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	msgBytes := make([]byte, 65536)
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

// ReadMessageRaw reads a raw JSON message from the relay
func (c *TestClient) ReadMessageRaw(timeout time.Duration) json.RawMessage {
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	msgBytes := make([]byte, 65536)
	n, err := c.conn.Read(msgBytes)
	if err != nil {
		c.t.Fatalf("Failed to read message: %v", err)
	}

	return json.RawMessage(msgBytes[:n])
}

// ExpectOK reads messages until it gets an OK for the given event ID.
// Returns (accepted bool, message string).
func (c *TestClient) ExpectOK(eventID string, timeout time.Duration) (bool, string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg := c.ReadMessage(time.Until(deadline))
		if len(msg) >= 4 && msg[0] == "OK" {
			if id, ok := msg[1].(string); ok && id == eventID {
				accepted, _ := msg[2].(bool)
				reason, _ := msg[3].(string)
				return accepted, reason
			}
		}
	}
	c.t.Fatalf("Timed out waiting for OK response for event %s", eventID)
	return false, ""
}

// ExpectEOSE reads messages until it gets an EOSE for the given subscription ID.
// Returns any EVENT messages received before the EOSE.
func (c *TestClient) ExpectEOSE(subID string, timeout time.Duration) []map[string]interface{} {
	var events []map[string]interface{}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg := c.ReadMessage(time.Until(deadline))
		if len(msg) >= 2 {
			if msg[0] == "EOSE" {
				if sid, ok := msg[1].(string); ok && sid == subID {
					return events
				}
			}
			if msg[0] == "EVENT" && len(msg) >= 3 {
				if sid, ok := msg[1].(string); ok && sid == subID {
					if evtMap, ok := msg[2].(map[string]interface{}); ok {
						events = append(events, evtMap)
					}
				}
			}
		}
	}
	c.t.Fatalf("Timed out waiting for EOSE for subscription %s", subID)
	return nil
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

// RandomSubID generates a random subscription ID for tests.
func RandomSubID() string {
	return fmt.Sprintf("test-%d", rand.Intn(1000000))
}
