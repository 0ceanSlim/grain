package core

import (
	"context"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"golang.org/x/net/websocket"
)

// Subscribe must dial its target relays concurrently, so a handful of relays
// (e.g. an author's outbox) come up in roughly one dial's time, not the sum —
// and a slow/dead relay can't stall the others. Also asserts leases are taken
// while active and fully released on Close.
func TestSubscribeDialsRelaysConcurrently(t *testing.T) {
	c := NewClient(DefaultConfig())
	c.relayPool.dialFn = func(string, time.Duration) (*websocket.Conn, error) {
		time.Sleep(150 * time.Millisecond) // simulate dial latency
		return nil, nil
	}
	c.relayPool.startReader = func(*RelayConnection) {}

	relays := []string{"wss://r1", "wss://r2", "wss://r3", "wss://r4"}

	start := time.Now()
	sub, err := c.Subscribe(context.Background(), []nostr.Filter{{Kinds: []int{0}}}, relays)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 450*time.Millisecond {
		t.Fatalf("Subscribe took %s — dials look serial (4x150ms); should be concurrent (~150ms)", elapsed)
	}

	for _, r := range relays {
		if l := leasesOf(c.relayPool, r); l != 1 {
			t.Fatalf("relay %s leases = %d while active, want 1", r, l)
		}
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, r := range relays {
		if l := leasesOf(c.relayPool, r); l != 0 {
			t.Fatalf("relay %s leases after Close = %d, want 0", r, l)
		}
	}
}
