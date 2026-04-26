package server

import (
	"context"
	"testing"
	"time"
)

// TestRegisterConnectionDoesNotDeadlockUnderMemoryPressure exercises the
// production lockup path: when RegisterConnection's memory check fires,
// the eviction must happen OUTSIDE cm.mu — otherwise the close path
// re-enters via RemoveConnection and self-deadlocks the goroutine,
// taking the lock down with it and freezing every subsequent connect.
//
// Uses the global connManager because that's what CloseClient calls
// into — a fresh local manager would let CloseClient's RemoveConnection
// touch a different mutex and the regression would slip past.
//
// If anyone reverts the lock-split fix in connectionManager.go, this
// test blocks forever (caught by the 2s timeout) instead of returning.
func TestRegisterConnectionDoesNotDeadlockUnderMemoryPressure(t *testing.T) {
	// Snapshot and restore global state so this test doesn't poison
	// other tests in the package.
	origThreshold := connManager.memoryThreshold
	origConns := connManager.connections
	t.Cleanup(func() {
		connManager.mu.Lock()
		connManager.memoryThreshold = origThreshold
		connManager.connections = origConns
		connManager.mu.Unlock()
	})

	connManager.mu.Lock()
	connManager.memoryThreshold = 0 // forces eviction every time
	connManager.connections = make(map[*Client]time.Time)
	connManager.mu.Unlock()

	// Pre-populate one client so removeOldestLocked has something to evict.
	octx, ocancel := context.WithCancel(context.Background())
	old := &Client{
		id:       "old",
		ctx:      octx,
		cancel:   ocancel,
		outgoing: make(chan []byte, clientOutgoingBuffer),
	}
	connManager.mu.Lock()
	connManager.connections[old] = time.Now().Add(-time.Hour)
	connManager.mu.Unlock()

	nctx, ncancel := context.WithCancel(context.Background())
	newClient := &Client{
		id:       "new",
		ctx:      nctx,
		cancel:   ncancel,
		outgoing: make(chan []byte, clientOutgoingBuffer),
	}

	done := make(chan struct{})
	go func() {
		connManager.RegisterConnection(newClient)
		close(done)
	}()

	select {
	case <-done:
		// Expected: returned promptly without deadlocking on cm.mu.
	case <-time.After(2 * time.Second):
		t.Fatal("RegisterConnection deadlocked under memory pressure (regression of the production WebSocket lockup)")
	}
}
