package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// NIP-50 fulltext search. nostrdb indexes content for kinds 1 and
// 30023 only — these tests use kind 1 throughout. Searches use a
// per-test unique token so concurrent test runs (and stale data from
// prior runs against the same default-relay container) don't leak
// matches into each other.

func uniqueToken(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("grntest%d", time.Now().UnixNano())
}

func TestNIP50_BasicMatch(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	tok := uniqueToken(t)
	matchEvt := kp.SignEvent(1, "hello "+tok+" world", nil)
	c.SendEvent(matchEvt)
	if ok, reason := c.ExpectOK(matchEvt.ID, 3*time.Second); !ok {
		t.Fatalf("publish rejected: %q", reason)
	}
	noMatch := kp.SignEvent(1, "totally unrelated content", nil)
	c.SendEvent(noMatch)
	if ok, reason := c.ExpectOK(noMatch.ID, 3*time.Second); !ok {
		t.Fatalf("publish rejected: %q", reason)
	}

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{"search": tok})
	got := c.ExpectEOSE(sub, 3*time.Second)

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 search match, got %d", len(got))
	}
	if id, _ := got[0]["id"].(string); id != matchEvt.ID {
		t.Errorf("expected matchEvt.ID, got %q", id)
	}
}

func TestNIP50_NoMatch(t *testing.T) {
	c := tests.NewTestClient(t)
	defer c.Close()

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{"search": uniqueToken(t)})

	got := c.ExpectEOSE(sub, 3*time.Second)
	if len(got) != 0 {
		t.Fatalf("expected zero results for unmatched search, got %d", len(got))
	}
}

func TestNIP50_CombinedWithAuthor(t *testing.T) {
	tok := uniqueToken(t)
	authorA := tests.NewTestKeypair()
	authorB := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	evtA := authorA.SignEvent(1, "from A: "+tok, nil)
	c.SendEvent(evtA)
	if ok, _ := c.ExpectOK(evtA.ID, 3*time.Second); !ok {
		t.Fatalf("authorA publish rejected")
	}
	evtB := authorB.SignEvent(1, "from B: "+tok, nil)
	c.SendEvent(evtB)
	if ok, _ := c.ExpectOK(evtB.ID, 3*time.Second); !ok {
		t.Fatalf("authorB publish rejected")
	}

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{
		"search":  tok,
		"authors": []string{authorA.PubKey},
	})

	got := c.ExpectEOSE(sub, 3*time.Second)
	if len(got) != 1 {
		t.Fatalf("expected 1 match constrained to authorA, got %d", len(got))
	}
	if id, _ := got[0]["id"].(string); id != evtA.ID {
		t.Errorf("expected evtA.ID, got %q", id)
	}
}

// Paging beyond nostrdb's MAX_TEXT_SEARCH_RESULTS=128 is handled by
// pagedTextSearch in handlers/req.go (Until-cursor loop, same shape as
// CountFiltered and the expiration bootstrap). Verifying it
// end-to-end requires the test client to consume >128 EVENT frames in
// rapid succession, which the shared TestClient.ReadMessage helper
// can't do reliably — golang.org/x/net/websocket's Conn.Read is
// io.Reader-like (frame-chunked), and the helper's single-Read +
// json.Unmarshal pattern occasionally hands back a partial frame
// under high-burst delivery. Reworking shared helpers to use
// websocket.Message.Receive is out of scope for this NIP.
//
// Paging logic remains exercised at runtime by any production client
// requesting more than 128 search matches.
