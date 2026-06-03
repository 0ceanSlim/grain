package server

import (
	"context"
	"testing"
	"time"
)

// TestPrintStatsStopsOnContextCancel is the representative regression test for
// #93: per-instance background loops must return when their context is
// cancelled instead of running for the life of the process and accumulating
// across config-reload restarts.
//
// PrintStats is the loop that runs in the caller's goroutine, so its return is
// directly observable. Every other background loop changed in this patch
// (pubkey-cache refresh, IP-ban sweeper, event purging, expiration sweeper,
// rejection aggregator, bucket cleanup, and the client-side cleanup/health
// goroutines) uses the identical `select { case <-ctx.Done(): return; ... }`
// shape.
func TestPrintStatsStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		PrintStats(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Returned promptly on cancellation — correct.
	case <-time.After(2 * time.Second):
		t.Fatal("PrintStats did not return after context cancellation")
	}
}
