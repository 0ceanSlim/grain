package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-eventpurge (port 8188) with:
//   purge_interval_minutes: 1
//   keep_interval_hours:    0
//   purge_by_kind_enabled:  true
//   kinds_to_purge:         [1]
//   purge_by_category:      regular: true
//
// Because grain's purge scheduler only supports minute-granularity intervals,
// this test waits ~70s for the first purge sweep. It is skipped in -short
// mode so the main integration run stays fast.

func TestEventPurge_ByKind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow purge test in -short mode")
	}

	kp := tests.NewTestKeypair()
	pub := tests.NewTestClientAt(t, tests.EventPurgeRelayURL)

	// Publish a kind-1 event (flagged for purging).
	evt := kp.SignEvent(1, "should be purged", nil)
	pub.SendEvent(evt)
	ok, reason := pub.ExpectOK(evt.ID, 5*time.Second)
	if !ok {
		pub.Close()
		t.Fatalf("initial publish rejected: %q", reason)
	}
	pub.Close()

	// Wait past one purge interval. The relay's read_timeout is 60s so we
	// can't hold the publish connection open across the sleep — close it
	// and reconnect for the query.
	t.Log("waiting 70s for purge sweep…")
	time.Sleep(70 * time.Second)

	// Query the event back — it should be gone.
	client := tests.NewTestClientAt(t, tests.EventPurgeRelayURL)
	defer client.Close()
	sub := tests.RandomSubID()
	client.Subscribe(sub, map[string]interface{}{"ids": []string{evt.ID}})
	events := client.ExpectEOSE(sub, 5*time.Second)
	if len(events) != 0 {
		t.Fatalf("expected event to be purged, still got %d results", len(events))
	}
}
