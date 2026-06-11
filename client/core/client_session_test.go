package core

import (
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func assertHasRelay(t *testing.T, urls []string, want string) {
	t.Helper()
	for _, u := range urls {
		if u == want {
			return
		}
	}
	t.Fatalf("expected %q in connected relays %v", want, urls)
}

// Logging in / switching users must be ADDITIVE: the shared pool (and the
// pinned index relays) survive a session switch, the previous user's relays
// are released, and the new user's relays are leased for the session. This is
// the core behavior change of Phase A2 — the old code tore the whole pool down.
func TestSessionRelaySwitchIsAdditive(t *testing.T) {
	c := NewClient(DefaultConfig())
	c.relayPool.dialFn = func(string, time.Duration) (*websocket.Conn, error) { return nil, nil }
	c.relayPool.startReader = func(*RelayConnection) {}

	// A pinned, connected index relay (as the manager sets up at startup).
	c.PinRelays("wss://index")
	if _, err := c.relayPool.Acquire("wss://index"); err != nil {
		t.Fatalf("acquire index: %v", err)
	}
	c.relayPool.Release("wss://index") // pinned ⇒ stays connected at lease 0

	// Login as user A.
	if err := c.SwitchToUserRelays([]RelayConfig{{URL: "wss://userA", Read: true, Write: true}}); err != nil {
		t.Fatalf("switch to user A: %v", err)
	}
	conns := c.GetConnectedRelays()
	assertHasRelay(t, conns, "wss://index") // index NOT torn down
	assertHasRelay(t, conns, "wss://userA")
	if l := leasesOf(c.relayPool, "wss://userA"); l != 1 {
		t.Fatalf("userA leases = %d, want 1 (held for the session)", l)
	}

	// Switch to user B: A's session lease released, B acquired.
	if err := c.SwitchToUserRelays([]RelayConfig{{URL: "wss://userB"}}); err != nil {
		t.Fatalf("switch to user B: %v", err)
	}
	if l := leasesOf(c.relayPool, "wss://userA"); l != 0 {
		t.Fatalf("userA leases after switch = %d, want 0 (released)", l)
	}
	if l := leasesOf(c.relayPool, "wss://userB"); l != 1 {
		t.Fatalf("userB leases = %d, want 1", l)
	}

	// Logout: session relays released, pinned index relay remains.
	if err := c.SwitchToIndexRelays(); err != nil {
		t.Fatalf("switch to index: %v", err)
	}
	if l := leasesOf(c.relayPool, "wss://userB"); l != 0 {
		t.Fatalf("userB leases after logout = %d, want 0", l)
	}
	assertHasRelay(t, c.GetConnectedRelays(), "wss://index")
}
