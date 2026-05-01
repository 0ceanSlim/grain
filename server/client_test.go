package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// stubClient builds a *Client suitable for SendMessageBlocking tests.
// Only the fields the function reads (ctx, id, outgoing) are populated;
// ws stays nil since SendMessageBlocking does not touch it.
func stubClient(t *testing.T, bufSize int) (*Client, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		id:       "stub",
		ctx:      ctx,
		cancel:   cancel,
		outgoing: make(chan []byte, bufSize),
	}, cancel
}

// TestSendMessageBlocking_SucceedsUnderCapacity confirms the simple
// happy path: small message, plenty of buffer, returns nil and the
// message lands on the channel.
func TestSendMessageBlocking_SucceedsUnderCapacity(t *testing.T) {
	c, cancel := stubClient(t, 4)
	defer cancel()

	for i := 0; i < 4; i++ {
		if err := c.SendMessageBlocking([]interface{}{"EVENT", "sub", "msg"}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if got := len(c.outgoing); got != 4 {
		t.Errorf("outgoing has %d messages, want 4", got)
	}
}

// TestSendMessageBlocking_BlocksThenUnblocks reproduces the production
// scenario: the buffer is full, the producer must wait until a slot
// frees up. This is the load-bearing assertion for the slow-consumer
// fix — without backpressure, a 500-event REQ overflowed the prior
// 256-slot buffer and disconnected healthy clients mid-fulfillment.
func TestSendMessageBlocking_BlocksThenUnblocks(t *testing.T) {
	c, cancel := stubClient(t, 2)
	defer cancel()

	// Pre-fill the buffer.
	for i := 0; i < 2; i++ {
		if err := c.SendMessageBlocking([]interface{}{"EVENT", "x"}); err != nil {
			t.Fatalf("prefill %d: %v", i, err)
		}
	}

	// Third send must block — verify by racing a "drain after delay"
	// against a "send completed" signal. If the send returns before
	// the drain, the function is not actually blocking.
	sent := make(chan time.Time, 1)
	go func() {
		_ = c.SendMessageBlocking([]interface{}{"EVENT", "third"})
		sent <- time.Now()
	}()

	// Give the goroutine time to enter the blocking select.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-sent:
		t.Fatal("SendMessageBlocking returned before buffer was drained")
	default:
	}

	// Drain one slot; the blocked send must complete.
	drainAt := time.Now()
	<-c.outgoing

	select {
	case completedAt := <-sent:
		if completedAt.Before(drainAt) {
			t.Errorf("send completed (%v) before drain (%v); should have been blocked",
				completedAt, drainAt)
		}
	case <-time.After(time.Second):
		t.Fatal("SendMessageBlocking did not unblock after slot drained")
	}
}

// TestSendMessageBlocking_ReturnsOnContextCancel ensures a blocked send
// can be unstuck by cancelling the client's context — this is what
// rescues the REQ-fulfillment loop when the peer genuinely vanishes
// (writeLoop exits, idle-timeout fires, etc.).
func TestSendMessageBlocking_ReturnsOnContextCancel(t *testing.T) {
	c, cancel := stubClient(t, 1)

	// Fill the buffer so the next send blocks.
	if err := c.SendMessageBlocking([]interface{}{"EVENT", "x"}); err != nil {
		t.Fatalf("prefill: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- c.SendMessageBlocking([]interface{}{"EVENT", "y"}) }()

	// Confirm the goroutine is actually blocked.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("SendMessageBlocking returned before ctx cancel")
	default:
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, errClientGone) {
			t.Errorf("expected errClientGone, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SendMessageBlocking did not return after ctx cancel")
	}
}

// TestSendMessageBlocking_AlreadyCancelledFastPath verifies the
// pre-marshal ctx check — when the connection is already gone there
// is no point doing json.Marshal work, and the channel must remain
// untouched.
func TestSendMessageBlocking_AlreadyCancelledFastPath(t *testing.T) {
	c, cancel := stubClient(t, 1)
	cancel() // already cancelled before any send

	if err := c.SendMessageBlocking([]interface{}{"EVENT", "x"}); !errors.Is(err, errClientGone) {
		t.Fatalf("expected errClientGone, got %v", err)
	}
	if got := len(c.outgoing); got != 0 {
		t.Errorf("outgoing has %d messages, want 0 (no enqueue on already-cancelled)", got)
	}
}

// TestSendMessageBlocking_RegressionLargeBatch is the canonical
// regression for the production bug: a batch of N > buffer messages
// must all enqueue successfully when a consumer drains in parallel.
// Before the fix, the synchronous loop in req.go's REQ handler would
// overflow at message buffer+1 and disconnect the client. With the
// blocking variant, the loop simply runs at the consumer's pace.
func TestSendMessageBlocking_RegressionLargeBatch(t *testing.T) {
	const bufSize = 4
	const total = 50

	c, cancel := stubClient(t, bufSize)
	defer cancel()

	// Background drainer simulates writeLoop. Slow enough that without
	// backpressure the producer would flood the buffer many times over.
	received := 0
	var mu sync.Mutex
	drainerDone := make(chan struct{})
	go func() {
		defer close(drainerDone)
		for {
			select {
			case <-c.outgoing:
				mu.Lock()
				received++
				mu.Unlock()
				time.Sleep(time.Millisecond) // pretend network latency
			case <-c.ctx.Done():
				return
			}
		}
	}()

	for i := 0; i < total; i++ {
		if err := c.SendMessageBlocking([]interface{}{"EVENT", "sub", i}); err != nil {
			t.Fatalf("send %d failed: %v (this is the production bug regressing)", i, err)
		}
	}

	// All messages must reach the consumer.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := received
		mu.Unlock()
		if got >= total {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-drainerDone

	mu.Lock()
	defer mu.Unlock()
	if received < total {
		t.Errorf("drainer received %d, want %d — messages lost or producer disconnected", received, total)
	}
}
