package core

import (
	"context"
	"errors"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// newTestSub builds a Subscription with just the channels collectLatestReplaceable
// reads. No live websocket or relay pool is needed — the helper only consumes
// Events / EOSE / Errors, which lets us drive it deterministically.
func newTestSub(id string, relays int) *Subscription {
	return &Subscription{
		ID:     id,
		Events: make(chan *nostr.Event, 10),
		Errors: make(chan error, 10),
		EOSE:   make(chan string, relays),
	}
}

// An event in hand plus a relay EOSE must return that event (never nil). The
// relay count is 2 so a single EOSE can't trip the all-EOSE→nil path.
func TestCollectReturnsEventOnEOSE(t *testing.T) {
	sub := newTestSub("t1", 2)
	sub.Events <- &nostr.Event{ID: "e1", CreatedAt: 100}
	sub.EOSE <- "wss://a"

	got := collectLatestReplaceable(context.Background(), sub, 2, 300*time.Millisecond)
	if got == nil || got.ID != "e1" {
		t.Fatalf("want event e1, got %+v", got)
	}
}

// Newest-by-created_at wins. No EOSE is sent, so both buffered events are
// drained before the deadline returns the latest — order-independent.
func TestCollectPicksHighestCreatedAt(t *testing.T) {
	sub := newTestSub("t2", 2)
	sub.Events <- &nostr.Event{ID: "old", CreatedAt: 100}
	sub.Events <- &nostr.Event{ID: "new", CreatedAt: 200}

	got := collectLatestReplaceable(context.Background(), sub, 2, 150*time.Millisecond)
	if got == nil || got.ID != "new" {
		t.Fatalf("want newest event 'new', got %+v", got)
	}
}

// This is the #77 regression guard: every relay reporting EOSE with no event
// must return nil *promptly*, not after waiting out the full deadline. If the
// old sub.Done bug ever returns, EOSE is never consumed and this blocks until
// the 2s deadline, failing the elapsed check.
func TestCollectAllEOSEReturnsNilPromptly(t *testing.T) {
	sub := newTestSub("t3", 2)
	sub.EOSE <- "wss://a"
	sub.EOSE <- "wss://b"

	start := time.Now()
	got := collectLatestReplaceable(context.Background(), sub, 2, 2*time.Second)
	elapsed := time.Since(start)

	if got != nil {
		t.Fatalf("want nil when all relays EOSE with no event, got %+v", got)
	}
	if elapsed > time.Second {
		t.Fatalf("all-EOSE should short-circuit, took %s (did it wait for the deadline?)", elapsed)
	}
}

// Nothing arrives: nil at the deadline (offline / not-found case).
func TestCollectTimeoutReturnsNil(t *testing.T) {
	sub := newTestSub("t4", 1)
	if got := collectLatestReplaceable(context.Background(), sub, 1, 150*time.Millisecond); got != nil {
		t.Fatalf("want nil on timeout, got %+v", got)
	}
}

// An event with no EOSE is still returned once the deadline hits.
func TestCollectTimeoutReturnsLatest(t *testing.T) {
	sub := newTestSub("t5", 2)
	sub.Events <- &nostr.Event{ID: "e", CreatedAt: 100}
	if got := collectLatestReplaceable(context.Background(), sub, 2, 150*time.Millisecond); got == nil || got.ID != "e" {
		t.Fatalf("want event e at timeout, got %+v", got)
	}
}

// A relay error is non-fatal: a later event still resolves the call.
func TestCollectErrorIsNonFatal(t *testing.T) {
	sub := newTestSub("t6", 2)
	sub.Errors <- errors.New("relay blew up")
	sub.Events <- &nostr.Event{ID: "e", CreatedAt: 100}

	if got := collectLatestReplaceable(context.Background(), sub, 2, 150*time.Millisecond); got == nil || got.ID != "e" {
		t.Fatalf("want event e despite a relay error, got %+v", got)
	}
}
