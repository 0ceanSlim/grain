package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// These tests exercise grain's NIP-09 deletion path end-to-end against the
// grain-default scenario relay. They rely on the vendored nostrdb fork that
// exposes ndb_request_delete_note — once a kind-5 is accepted and processed,
// the referenced event must be physically gone from the database, not just
// hidden at query time.

// publishDeleteEvent sends a kind-5 referencing `targetID` via an `e` tag
// and waits for the OK. Returns the OK reason on rejection so callers can
// fail with context.
func publishDelete(t *testing.T, c *tests.TestClient, kp *tests.TestKeypair, tag []string) string {
	t.Helper()
	del := kp.SignEvent(5, "delete test", [][]string{tag})
	c.SendEvent(del)
	ok, reason := c.ExpectOK(del.ID, 3*time.Second)
	if !ok {
		t.Fatalf("kind-5 publish rejected: %q", reason)
	}
	return del.ID
}

func TestNIP09_DeleteByID(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// Publish an event.
	evt := kp.SignEvent(1, "delete me please", nil)
	c.SendEvent(evt)
	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("initial publish rejected: %q", reason)
	}

	// Verify it's queryable.
	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{"ids": []string{evt.ID}})
	if got := c.ExpectEOSE(sub, 3*time.Second); len(got) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(got))
	}

	// Publish a kind-5 deleting it.
	delID := publishDelete(t, c, kp, []string{"e", evt.ID})

	// Query the target — should be gone.
	sub2 := tests.RandomSubID()
	c.Subscribe(sub2, map[string]interface{}{"ids": []string{evt.ID}})
	if got := c.ExpectEOSE(sub2, 3*time.Second); len(got) != 0 {
		t.Fatalf("expected event to be deleted, got %d results", len(got))
	}

	// The kind-5 record itself must still be visible (NIP-09 spec).
	sub3 := tests.RandomSubID()
	c.Subscribe(sub3, map[string]interface{}{"ids": []string{delID}})
	if got := c.ExpectEOSE(sub3, 3*time.Second); len(got) != 1 {
		t.Fatalf("expected kind-5 marker to remain visible, got %d", len(got))
	}
}

func TestNIP09_RejectsCrossAuthorDelete(t *testing.T) {
	alice := tests.NewTestKeypair()
	mallory := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// Alice publishes an event.
	evt := alice.SignEvent(1, "alice's note", nil)
	c.SendEvent(evt)
	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("alice publish rejected: %q", reason)
	}

	// Mallory tries to delete it with her own kind-5. The kind-5 itself is
	// accepted and stored (grain stores the record regardless), but the
	// target event must NOT be removed — NIP-09 requires same-author.
	publishDelete(t, c, mallory, []string{"e", evt.ID})

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{"ids": []string{evt.ID}})
	got := c.ExpectEOSE(sub, 3*time.Second)
	if len(got) != 1 {
		t.Fatalf("cross-author delete should be rejected: expected 1 result, got %d", len(got))
	}
}

func TestNIP09_DeleteAddressable(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// Publish an addressable kind-30000 with a d-tag.
	dTag := "delete-me-" + tests.RandomSubID()
	evt := kp.SignEvent(30000, "addressable payload", [][]string{{"d", dTag}})
	c.SendEvent(evt)
	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("addressable publish rejected: %q", reason)
	}

	// Delete it by a-tag coordinate.
	coord := "30000:" + kp.PubKey + ":" + dTag
	publishDelete(t, c, kp, []string{"a", coord})

	// Verify the addressable event is gone.
	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{30000},
		"#d":      []string{dTag},
	})
	if got := c.ExpectEOSE(sub, 3*time.Second); len(got) != 0 {
		t.Fatalf("expected addressable event deleted, got %d", len(got))
	}
}
