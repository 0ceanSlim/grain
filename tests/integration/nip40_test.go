package integration

import (
	"strconv"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// NIP-40 expiration coverage against the grain-default scenario relay.
//
//   - Past-dated `expiration` tags are rejected at ingest with an
//     "invalid: event is expired" reason.
//   - Future-dated expirations are accepted and queryable until they pass.
//   - Once the expiration is in the past, REQ no longer returns the event
//     (the sweeper may or may not have physically deleted it yet — the
//     visible-behavior contract is what matters here).

func expirationTag(ts int64) []string {
	return []string{"expiration", strconv.FormatInt(ts, 10)}
}

func TestNIP40_RejectAlreadyExpired(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	expired := time.Now().Unix() - 60
	evt := kp.SignEvent(1, "should be rejected", [][]string{expirationTag(expired)})
	c.SendEvent(evt)

	ok, reason := c.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected expired event to be rejected, got OK true (reason=%q)", reason)
	}
	if !tests.ContainsAny(reason, "expired", "invalid") {
		t.Errorf("unexpected reject reason: %q", reason)
	}
}

func TestNIP40_AcceptFutureExpiration(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	future := time.Now().Unix() + 600
	evt := kp.SignEvent(1, "expires later", [][]string{expirationTag(future)})
	c.SendEvent(evt)

	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("future-expiration event was rejected: %q", reason)
	}

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{"ids": []string{evt.ID}})
	if got := c.ExpectEOSE(sub, 3*time.Second); len(got) != 1 {
		t.Fatalf("expected 1 result for unexpired event, got %d", len(got))
	}
}

func TestNIP40_NotReturnedAfterExpiration(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// Pick a window short enough to keep the test fast but long enough
	// that the relay has time to ingest before it expires.
	expireIn := 3 * time.Second
	expireAt := time.Now().Add(expireIn).Unix()

	evt := kp.SignEvent(1, "ephemeral", [][]string{expirationTag(expireAt)})
	c.SendEvent(evt)
	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("publish rejected: %q", reason)
	}

	// Confirm it's visible before expiration.
	subBefore := tests.RandomSubID()
	c.Subscribe(subBefore, map[string]interface{}{"ids": []string{evt.ID}})
	if got := c.ExpectEOSE(subBefore, 3*time.Second); len(got) != 1 {
		t.Fatalf("expected 1 result before expiration, got %d", len(got))
	}

	// Sleep until past the expiration timestamp + a small buffer.
	time.Sleep(expireIn + 2*time.Second)

	subAfter := tests.RandomSubID()
	c.Subscribe(subAfter, map[string]interface{}{"ids": []string{evt.ID}})
	if got := c.ExpectEOSE(subAfter, 3*time.Second); len(got) != 0 {
		t.Fatalf("expected expired event to be filtered from REQ, got %d results", len(got))
	}
}
