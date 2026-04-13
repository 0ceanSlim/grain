package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-eventpurge (port 8188) with:
//   purge_interval_minutes: 1
//   keep_interval_hours:    0
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
// This exercises both v0.5 delete AND v0.4 purge_by_category compat:
// the kind-0 replaceable event must survive because its category is
// configured as `replaceable: false`, even though its created_at is
// past the cutoff and it isn't author-whitelisted.

func TestEventPurge_CategoryGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow purge test in -short mode")
	}

	kp := tests.NewTestKeypair()
	pub := tests.NewTestClientAt(t, tests.EventPurgeRelayURL)

	// Publish a kind-1 (regular -> should be purged).
	evtRegular := kp.SignEvent(1, "should be purged", nil)
	pub.SendEvent(evtRegular)
	if ok, reason := pub.ExpectOK(evtRegular.ID, 5*time.Second); !ok {
		pub.Close()
		t.Fatalf("kind-1 publish rejected: %q", reason)
	}

	// Publish a kind-0 (replaceable -> should be KEPT by category gate).
	evtReplaceable := kp.SignEvent(0, `{"name":"keepme"}`, nil)
	pub.SendEvent(evtReplaceable)
	if ok, reason := pub.ExpectOK(evtReplaceable.ID, 5*time.Second); !ok {
		pub.Close()
		t.Fatalf("kind-0 publish rejected: %q", reason)
	}
	pub.Close()

	// Wait past one purge interval. Relay read_timeout is 60s so we must
	// close the publish connection across the sleep.
	t.Log("waiting 70s for purge sweep…")
	time.Sleep(70 * time.Second)

	client := tests.NewTestClientAt(t, tests.EventPurgeRelayURL)
	defer client.Close()

	// kind-1 (regular) should be gone.
	subReg := tests.RandomSubID()
	client.Subscribe(subReg, map[string]interface{}{"ids": []string{evtRegular.ID}})
	if got := client.ExpectEOSE(subReg, 5*time.Second); len(got) != 0 {
		t.Fatalf("expected kind-1 event to be purged, still got %d results", len(got))
	}

	// kind-0 (replaceable) should survive — category gate blocks the delete.
	subRepl := tests.RandomSubID()
	client.Subscribe(subRepl, map[string]interface{}{"ids": []string{evtReplaceable.ID}})
	if got := client.ExpectEOSE(subRepl, 5*time.Second); len(got) != 1 {
		t.Fatalf("expected kind-0 replaceable to be kept by category gate, got %d results", len(got))
	}
}
