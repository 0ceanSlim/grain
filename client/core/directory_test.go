package core

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestParseDMRelays(t *testing.T) {
	ev := &nostr.Event{Kind: 10050, Tags: [][]string{
		{"relay", "wss://a"},
		{"relay", "wss://b"},
		{"other", "ignored"},
		{"relay"}, // malformed (no url) — skipped
	}}
	got := parseDMRelays(ev)
	if len(got) != 2 || got[0] != "wss://a" || got[1] != "wss://b" {
		t.Fatalf("parseDMRelays = %v, want [wss://a wss://b]", got)
	}
}

func TestAppendUniqueDedupsPreservingOrder(t *testing.T) {
	got := appendUnique([]string{"a", "b"}, []string{"b", "c", "a"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("appendUnique = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("appendUnique = %v, want %v", got, want)
		}
	}
}

func TestKnownRelays(t *testing.T) {
	d := newRelayDirectory(time.Hour, time.Minute, func(string) *UserRelays { return nil })
	d.entries["a"] = &UserRelays{Outbox: []string{"wss://o1"}, Inbox: []string{"wss://i1"}}
	d.entries["b"] = &UserRelays{Outbox: []string{"wss://o1", "wss://o2"}, DMInbox: []string{"wss://dm"}}

	// Distinct across both entries: o1, i1, o2, dm = 4.
	if got := d.KnownRelays(); len(got) != 4 {
		t.Fatalf("KnownRelays = %v, want 4 distinct", got)
	}
}

func TestPoolStatsKnownCountsDefaultsAndDirectory(t *testing.T) {
	c := newClientWithStubDirectory(map[string]*UserRelays{
		"a": {Outbox: []string{"wss://o1"}, Inbox: []string{"wss://i1"}},
		"b": {Outbox: []string{"wss://o2"}},
	})
	c.directory.Lookup("a")
	c.directory.Lookup("b")

	// 5 default index relays + 3 distinct resolved (o1, i1, o2) = 8.
	if s := c.PoolStats(); s.Known != 8 {
		t.Fatalf("Known = %d, want 8 (5 index + 3 resolved)", s.Known)
	}
}

func TestDirectoryCachesAndSingleFlights(t *testing.T) {
	var calls int32
	d := newRelayDirectory(time.Hour, time.Minute, func(string) *UserRelays {
		atomic.AddInt32(&calls, 1)
		time.Sleep(10 * time.Millisecond) // widen the single-flight window
		return &UserRelays{Outbox: []string{"wss://o"}, FetchedAt: time.Now()}
	})

	const n = 6
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); d.Lookup("pk") }()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("single-flight: resolved %d times, want 1", got)
	}

	// A subsequent lookup is served from cache.
	d.Lookup("pk")
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("cache hit: resolved %d times, want 1", got)
	}
}

func TestDirectoryReresolvesAfterTTL(t *testing.T) {
	var calls int32
	d := newRelayDirectory(20*time.Millisecond, 20*time.Millisecond, func(string) *UserRelays {
		atomic.AddInt32(&calls, 1)
		return &UserRelays{Outbox: []string{"wss://o"}, FetchedAt: time.Now()}
	})
	d.Lookup("pk")
	time.Sleep(40 * time.Millisecond) // exceed the TTL
	d.Lookup("pk")
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("want 2 resolves after TTL expiry, got %d", got)
	}
}

func TestDirectoryNegativeCachedBriefly(t *testing.T) {
	var calls int32
	// Positive TTL long, negative TTL short: a "no list" answer must re-resolve
	// soon (the user may publish one) without hammering on every lookup.
	d := newRelayDirectory(time.Hour, 40*time.Millisecond, func(string) *UserRelays {
		atomic.AddInt32(&calls, 1)
		return nil // nothing found
	})

	ur := d.Lookup("pk")
	if !ur.Negative {
		t.Fatal("nil resolve should produce a Negative entry")
	}
	d.Lookup("pk") // within negTTL — cached
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("negative cache: resolved %d times, want 1", got)
	}

	time.Sleep(70 * time.Millisecond) // exceed negTTL
	d.Lookup("pk")
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("want re-resolve after negTTL, got %d", got)
	}
}
