package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-eventpurge (port 8188) with:
//   purge_interval_minutes: 1
//   keep_interval_hours:    1
//   retention_clock:        received
//   late_arrival_minutes:   600
//   purge_by_kind_enabled:  false    (category-gated purge only)
//   purge_by_category:
//     regular:     true   -> kind 1 purged
//     replaceable: false  -> kind 0 kept
//     addressable: false
//     deprecated:  true
//
// The grain purge scheduler only supports minute-granularity intervals,
// so this test waits ~70s for the first purge sweep. It is skipped in
// -short mode so the main integration run stays fast.
//
// Events are backdated 2h, past the 1h keep window. The kind-1 is under the
// 10h late-arrival threshold, so it is aged from created_at and purged. The
// kind-0 is kept by the category gate (v0.4 purge_by_category compat). A
// kind-1 backdated 20h is a late arrival: the received clock ages it from
// when the relay got it, so it survives the sweep.

func TestEventPurge_CategoryGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow purge test in -short mode")
	}

	kp := tests.NewTestKeypair()
	pub := tests.NewTestClientAt(t, tests.EventPurgeRelayURL)
	now := time.Now().Unix()

	publish := func(kind int, content string, createdAt int64) string {
		evt := kp.SignEventAt(kind, content, nil, createdAt)
		pub.SendEvent(evt)
		if ok, reason := pub.ExpectOK(evt.ID, 5*time.Second); !ok {
			pub.Close()
			t.Fatalf("kind-%d %q rejected: %q", kind, content, reason)
		}
		return evt.ID
	}
	regular := publish(1, "should be purged", now-2*3600)
	replaceable := publish(0, `{"name":"keepme"}`, now-2*3600)
	late := publish(1, "late arrival, should be kept", now-20*3600)
	pub.Close()

	// Wait past one purge interval. Relay read_timeout is 60s so we must
	// close the publish connection across the sleep.
	t.Log("waiting 70s for purge sweep…")
	time.Sleep(70 * time.Second)

	client := tests.NewTestClientAt(t, tests.EventPurgeRelayURL)
	defer client.Close()

	count := func(id string) int {
		sub := tests.RandomSubID()
		client.Subscribe(sub, map[string]interface{}{"ids": []string{id}})
		return len(client.ExpectEOSE(sub, 5*time.Second))
	}
	if n := count(regular); n != 0 {
		t.Errorf("expected the backdated kind-1 to be purged, still got %d results", n)
	}
	if n := count(replaceable); n != 1 {
		t.Errorf("expected kind-0 replaceable to be kept by category gate, got %d results", n)
	}
	if n := count(late); n != 1 {
		t.Errorf("expected the late arrival to be kept by the received clock, got %d results", n)
	}
}
