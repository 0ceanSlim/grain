package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"golang.org/x/net/websocket"
)

// TestSendToBackupRelayHappyPath stands up a local websocket server and
// verifies SendToBackupRelay connects and writes a NIP-01 ["EVENT", ...]
// frame. It guards the rewritten dial/write path from #95 (explicit dial
// config + write deadline, no trailing sleep).
func TestSendToBackupRelayHappyPath(t *testing.T) {
	received := make(chan string, 1)

	// Mirror grain's own relay server: skip the origin check so the test
	// client's handshake is accepted.
	srvHandler := websocket.Server{
		Handshake: func(*websocket.Config, *http.Request) error { return nil },
		Handler: websocket.Handler(func(ws *websocket.Conn) {
			var msg string
			if err := websocket.Message.Receive(ws, &msg); err == nil {
				received <- msg
			}
		}),
	}
	srv := httptest.NewServer(srvHandler)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	evt := nostr.Event{ID: "abc123", Kind: 1, PubKey: "deadbeef", Content: "hello backup"}
	if err := SendToBackupRelay(wsURL, evt); err != nil {
		t.Fatalf("SendToBackupRelay: %v", err)
	}

	select {
	case msg := <-received:
		if !strings.Contains(msg, `"EVENT"`) || !strings.Contains(msg, "abc123") {
			t.Fatalf("backup relay received unexpected frame: %s", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("backup relay did not receive the event frame")
	}
}

// TestSendToBackupRelayConfigError verifies a malformed URL fails fast with an
// error rather than hanging — the function must never block unbounded (#95).
func TestSendToBackupRelayConfigError(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- SendToBackupRelay("://not-a-url", nostr.Event{ID: "x", Kind: 1})
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error for malformed backup relay URL")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SendToBackupRelay blocked on a malformed URL instead of failing fast")
	}
}
