package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// These tests run against the grain-ratelimit service (port 8183) which is
// configured in tests/docker/configs/ratelimit.yml with deliberately tight
// limits.
//
// Configured values (must stay in sync with the YAML):
//   ws_limit/burst:            500 / 1000 (intentionally permissive — shared limiter)
//   event_limit/burst:         3 / 3
//   req_limit/burst:           2 / 2
//   max_event_size:            1024 B
//   kind 1 max_size:           256  B
//   category "regular" lim:    2 / 2
//   kind 7 lim:                1 / 1

func TestRateLimit_GlobalEvent(t *testing.T) {
	client := tests.NewTestClientAt(t, tests.RateLimitRelayURL)
	defer client.Close()

	// Use kind 3 (replaceable category limit is 10/20) so the first rejection
	// we hit is the *global* event_limit (3/3) rather than the regular category.
	// Each iteration uses a fresh keypair: kind 3 is replaceable, so sending
	// multiple kind-3 events from the same pubkey triggers the replaceable
	// dedup check before the rate limiter ever fires.
	var rejectMsg string
	for i := 0; i < 20; i++ {
		kp := tests.NewTestKeypair()
		evt := kp.SignEvent(3, fmt.Sprintf("rl-global-%d", i), nil)
		client.SendEvent(evt)
		ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
		if !ok {
			rejectMsg = reason
			break
		}
	}
	if rejectMsg == "" {
		t.Fatal("expected at least one event to be rejected by global rate limit")
	}
	if !strings.Contains(rejectMsg, "Global event rate limit exceeded") {
		t.Fatalf("expected global rate limit reject, got %q", rejectMsg)
	}
}

func TestRateLimit_Category(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.RateLimitRelayURL)
	defer client.Close()

	// Kind 1 is "regular". Category limit 2/2, global limit 3/3 — so the
	// category should trip before global on the 3rd rapid send.
	var rejectMsg string
	for i := 0; i < 10; i++ {
		evt := kp.SignEvent(1, fmt.Sprintf("rl-cat-%d", i), nil)
		client.SendEvent(evt)
		ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
		if !ok {
			rejectMsg = reason
			break
		}
	}
	if rejectMsg == "" {
		t.Fatal("expected at least one event to be rejected")
	}
	if !tests.ContainsAny(rejectMsg,
		"Rate limit exceeded for category: regular",
		"Global event rate limit exceeded",
	) {
		t.Fatalf("expected category or global rate limit reject, got %q", rejectMsg)
	}
}

func TestRateLimit_Kind(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.RateLimitRelayURL)
	defer client.Close()

	// Kind 7 has its own 1/1 cap. Send two in a row: second should be rejected
	// by the kind limiter (category "regular" has 2/2 so kind fires first).
	var rejectMsg string
	for i := 0; i < 5; i++ {
		evt := kp.SignEvent(7, fmt.Sprintf("rl-kind-%d", i), nil)
		client.SendEvent(evt)
		ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
		if !ok {
			rejectMsg = reason
			break
		}
	}
	if rejectMsg == "" {
		t.Fatal("expected kind 7 to be rate limited")
	}
	if !tests.ContainsAny(rejectMsg,
		"Rate limit exceeded for kind: 7",
		"Rate limit exceeded for category: regular",
		"Global event rate limit exceeded",
	) {
		t.Fatalf("unexpected reject: %q", rejectMsg)
	}
}

func TestRateLimit_GlobalEventSize(t *testing.T) {
	// The global event_limit=3/sec bucket is a singleton shared across
	// every client on this scenario container. The preceding rate tests
	// drain it; wait for it to refill so the size check fires first
	// rather than the rate check.
	time.Sleep(2 * time.Second)

	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.RateLimitRelayURL)
	defer client.Close()

	// 2KB content > 1KB global max_event_size. Use kind 3 so per-kind size
	// limit doesn't fire first.
	big := strings.Repeat("x", 2048)
	evt := kp.SignEvent(3, big, nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected oversized event to be rejected")
	}
	if !strings.Contains(reason, "Global event size limit exceeded") {
		t.Fatalf("expected global size limit reject, got %q", reason)
	}
}

func TestRateLimit_KindSize(t *testing.T) {
	// See note in TestRateLimit_GlobalEventSize — refill the shared
	// global event bucket before running a single-event size check.
	time.Sleep(2 * time.Second)

	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.RateLimitRelayURL)
	defer client.Close()

	// Kind 1 capped at 256 B. 400 B content is under the 1KB global cap but
	// over the kind cap.
	content := strings.Repeat("y", 400)
	evt := kp.SignEvent(1, content, nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected kind-1 oversized event to be rejected")
	}
	if !strings.Contains(reason, "Event size limit exceeded for kind") {
		t.Fatalf("expected kind size limit reject, got %q", reason)
	}
}
