package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// expectCount reads frames until a COUNT for subID arrives, then returns
// (count, approximate). Fails the test on timeout.
func expectCount(t *testing.T, c *tests.TestClient, subID string, timeout time.Duration) (int, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg, err := c.TryReadMessage(time.Until(deadline))
		if err != nil {
			t.Fatalf("read failed waiting for COUNT %s: %v", subID, err)
		}
		if len(msg) >= 3 && msg[0] == "COUNT" {
			if sid, _ := msg[1].(string); sid == subID {
				payload, ok := msg[2].(map[string]interface{})
				if !ok {
					t.Fatalf("COUNT payload not an object: %v", msg[2])
				}
				cf, _ := payload["count"].(float64)
				approx, _ := payload["approximate"].(bool)
				return int(cf), approx
			}
		}
	}
	t.Fatalf("timeout waiting for COUNT %s", subID)
	return 0, false
}

func sendCount(c *tests.TestClient, subID string, filters ...map[string]interface{}) {
	msg := make([]interface{}, 2+len(filters))
	msg[0] = "COUNT"
	msg[1] = subID
	for i, f := range filters {
		msg[2+i] = f
	}
	c.SendMessage(msg)
}

func TestNIP45_CountByAuthor(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// Publish three events from the same author. Content must vary so
	// the per-event ID hash differs even when same-second timestamps
	// collide — otherwise the relay rejects the second/third as
	// duplicates.
	for i := 0; i < 3; i++ {
		evt := kp.SignEvent(1, fmt.Sprintf("count me %d", i), nil)
		c.SendEvent(evt)
		if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
			t.Fatalf("publish %d rejected: %q", i, reason)
		}
	}

	subID := tests.RandomSubID()
	sendCount(c, subID, map[string]interface{}{"authors": []string{kp.PubKey}, "kinds": []int{1}})

	count, _ := expectCount(t, c, subID, 3*time.Second)
	if count < 3 {
		t.Fatalf("expected >=3, got %d", count)
	}
}

func TestNIP45_CountEmptyResult(t *testing.T) {
	c := tests.NewTestClient(t)
	defer c.Close()

	// Random pubkey — nothing should match.
	missing := tests.NewTestKeypair()
	subID := tests.RandomSubID()
	sendCount(c, subID, map[string]interface{}{"authors": []string{missing.PubKey}})

	count, _ := expectCount(t, c, subID, 3*time.Second)
	if count != 0 {
		t.Fatalf("expected 0 matches for unknown author, got %d", count)
	}
}

func TestNIP45_CountMultiFilterApproximate(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	evt := kp.SignEvent(1, "multifilter", nil)
	c.SendEvent(evt)
	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("publish rejected: %q", reason)
	}

	subID := tests.RandomSubID()
	sendCount(c, subID,
		map[string]interface{}{"authors": []string{kp.PubKey}},
		map[string]interface{}{"kinds": []int{1}},
	)

	count, approximate := expectCount(t, c, subID, 3*time.Second)
	if count < 1 {
		t.Fatalf("expected >=1 from multi-filter union, got %d", count)
	}
	if !approximate {
		t.Errorf("expected approximate=true for multi-filter request, got exact")
	}
}
