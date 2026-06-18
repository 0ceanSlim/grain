package core

import (
	"context"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestStreamOptions(t *testing.T) {
	cfg := streamConfig{timeout: 10 * time.Second}
	for _, o := range []StreamOption{WithLimit(5), WithTimeout(2 * time.Second), WithLive()} {
		o(&cfg)
	}
	if cfg.limit != 5 {
		t.Errorf("limit = %d, want 5", cfg.limit)
	}
	if cfg.timeout != 2*time.Second {
		t.Errorf("timeout = %v, want 2s", cfg.timeout)
	}
	if !cfg.live {
		t.Error("live should be true")
	}
}

func TestStreamEventsNoRelays(t *testing.T) {
	c := NewClient(DefaultConfig())
	ch := c.StreamEvents(context.Background(), nostr.Filter{Kinds: []int{1}}, nil)

	// With no relays the channel must close promptly with nothing delivered.
	select {
	case ev, ok := <-ch:
		if ok {
			t.Errorf("expected closed channel, got event %v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamEvents with no relays did not close the channel")
	}

	if got := c.QueryEvents(context.Background(), nostr.Filter{Kinds: []int{1}}, nil); len(got) != 0 {
		t.Errorf("QueryEvents(no relays) = %v, want empty", got)
	}
}
