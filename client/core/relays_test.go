package core

import (
	"testing"
	"time"
)

// newTestRelayConn builds a RelayConnection with no live websocket (Conn is
// nil, so close() skips the socket close). It's enough to exercise the
// done-channel teardown coordination that #94 is about.
func newTestRelayConn() *RelayConnection {
	return &RelayConnection{
		URL:           "wss://test.invalid",
		Status:        StatusConnected,
		Subscriptions: map[string]bool{},
		writeChan:     make(chan []byte, 1),
		done:          make(chan struct{}),
	}
}

// TestCloseReleasesDoneWaiter is the #94 regression test: a goroutine blocked
// on done (as writeHandler is) must be released when the connection is closed,
// and close() must be safe to call more than once (the reconnect path in
// Connect calls it on an already-dead connection).
func TestCloseReleasesDoneWaiter(t *testing.T) {
	rc := newTestRelayConn()

	released := make(chan struct{})
	go func() {
		<-rc.done // mimics writeHandler's `case <-rc.done`
		close(released)
	}()

	if err := rc.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("waiter on done was not released by close()")
	}

	// Reconnect path calls close() again on the dead connection: must be a
	// safe no-op, not a double-close-of-channel panic.
	if err := rc.close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestReadSideExitReleasesWriter reproduces the exact #94 pathology: the read
// side exits first (signalDone, as the readHandler defer now does), which must
// release a writer blocked on done. A later close() (reconnect) must still be
// a safe no-op. Before the fix, the read-side exit closed nothing and close()
// short-circuited on Status, so the writer blocked forever.
func TestReadSideExitReleasesWriter(t *testing.T) {
	rc := newTestRelayConn()

	released := make(chan struct{})
	go func() {
		<-rc.done
		close(released)
	}()

	rc.setStatus(StatusDisconnected) // readHandler defer sets this...
	rc.signalDone()                  // ...and then signals done

	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("read-side exit did not release the writer (this was the #94 leak)")
	}

	if err := rc.close(); err != nil {
		t.Fatalf("close after read-side exit: %v", err)
	}
}

// TestSignalDoneIdempotent ensures repeated termination signals never panic
// (close of a closed channel) and leave done closed.
func TestSignalDoneIdempotent(t *testing.T) {
	rc := newTestRelayConn()
	rc.signalDone()
	rc.signalDone()
	rc.signalDone()
	select {
	case <-rc.done:
	default:
		t.Fatal("done should be closed after signalDone")
	}
}
