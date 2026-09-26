package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Malformed filter fields used to be dropped silently, widening the REQ into
// an unconstrained query (rc4 prod: {"kinds":["1"]} returned kind 7375s).
// Each must now be refused with CLOSED "invalid: ..." and no events.
func TestREQ_MalformedFilterRejected(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"string kind":    {"kinds": []interface{}{"1"}},
		"65-char author": {"authors": []string{strings.Repeat("0", 65)}},
		"non-hex id":     {"ids": []string{strings.Repeat("G", 64)}},
		"short non-hex":  {"ids": []string{"zz"}},
		"empty id":       {"ids": []string{""}},
		"negative limit": {"limit": -1},
		"string since":   {"since": "1700000000"},
		"tag not array":  {"#e": "abc"},
	}
	for name, filter := range cases {
		t.Run(name, func(t *testing.T) {
			c := tests.NewTestClient(t)
			defer c.Close()

			subID := tests.RandomSubID()
			c.Subscribe(subID, filter)
			for {
				msg := c.ReadMessage(5 * time.Second)
				switch msg[0] {
				case "EVENT", "EOSE":
					t.Fatalf("%v: got %s, want CLOSED invalid", filter, msg[0])
				case "CLOSED":
					if reason, _ := msg[2].(string); !strings.HasPrefix(reason, "invalid:") {
						t.Fatalf("%v: CLOSED reason %q, want invalid: prefix", filter, reason)
					}
					return
				}
			}
		})
	}
}

func TestCOUNT_MalformedFilterRejected(t *testing.T) {
	c := tests.NewTestClient(t)
	defer c.Close()

	subID := tests.RandomSubID()
	sendCount(c, subID, map[string]interface{}{"kinds": []interface{}{"1"}})
	if reason := c.ExpectClosed(subID, 5*time.Second); !strings.HasPrefix(reason, "invalid:") {
		t.Fatalf("CLOSED reason %q, want invalid: prefix", reason)
	}
}

// An empty list can never match: EOSE with no events, not a firehose.
func TestREQ_EmptyListMatchesNothing(t *testing.T) {
	kp := tests.NewTestKeypair()
	pub := tests.NewTestClient(t)
	defer pub.Close()
	evt := kp.SignEvent(1, "empty-list-probe", nil)
	pub.SendEvent(evt)
	pub.ExpectOK(evt.ID, 5*time.Second)

	c := tests.NewTestClient(t)
	defer c.Close()
	subID := tests.RandomSubID()
	c.Subscribe(subID, map[string]interface{}{"authors": []string{}})
	if events := c.ExpectEOSE(subID, 5*time.Second); len(events) != 0 {
		t.Fatalf("authors:[] returned %d events, want 0", len(events))
	}
}

// Prefix ids/authors are deliberate (glyphbyte.dev): the query still runs
// and returns the match, preceded by an advisory NOTICE — once per connection.
func TestREQ_PrefixMatchNotice(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	evt := kp.SignEvent(1, "prefix-notice-probe", nil)
	c.SendEvent(evt)
	c.ExpectOK(evt.ID, 5*time.Second)
	if !c.AwaitCommit(evt.ID, 5*time.Second) {
		t.Fatal("event not committed")
	}

	subID := tests.RandomSubID()
	c.Subscribe(subID, map[string]interface{}{"ids": []string{evt.ID[:7]}})

	var notice string
	found := false
	for {
		msg := c.ReadMessage(5 * time.Second)
		if msg[0] == "NOTICE" {
			notice, _ = msg[1].(string)
		}
		if msg[0] == "EVENT" && msg[1] == subID {
			if m, ok := msg[2].(map[string]interface{}); ok && m["id"] == evt.ID {
				found = true
			}
		}
		if msg[0] == "EOSE" && msg[1] == subID {
			break
		}
		if msg[0] == "CLOSED" {
			t.Fatalf("prefix REQ was refused: %v", msg)
		}
	}
	if !found {
		t.Fatalf("prefix %q did not return event %s", evt.ID[:7], evt.ID)
	}
	if !strings.Contains(notice, "prefixes") {
		t.Fatalf("NOTICE = %q, want the prefix-match advisory", notice)
	}

	// Second prefix REQ on the same connection: no repeat NOTICE.
	sub2 := tests.RandomSubID()
	c.Subscribe(sub2, map[string]interface{}{"authors": []string{evt.PubKey[:4]}})
	for {
		msg := c.ReadMessage(5 * time.Second)
		if msg[0] == "NOTICE" {
			t.Fatalf("prefix NOTICE repeated on the same connection: %v", msg)
		}
		if msg[0] == "EOSE" && msg[1] == sub2 {
			break
		}
	}
}

// At max_subscriptions_per_client (10 on the default fixture) a new REQ
// evicts the oldest sub and must tell the client with CLOSED.
func TestREQ_SubscriptionCapClosesOldest(t *testing.T) {
	c := tests.NewTestClient(t)
	defer c.Close()

	const maxSubs = 10
	ids := make([]string, maxSubs)
	for i := range ids {
		ids[i] = tests.RandomSubID()
		c.Subscribe(ids[i], map[string]interface{}{"kinds": []int{1}, "limit": 1})
		c.ExpectEOSE(ids[i], 5*time.Second)
		// Re-issuing the first sub before the cap moves it to the back of
		// the queue, so the second one is the oldest when the cap hits.
		if i == 1 {
			c.Subscribe(ids[0], map[string]interface{}{"kinds": []int{1}, "limit": 2})
			c.ExpectEOSE(ids[0], 5*time.Second)
		}
	}

	// The eviction's CLOSED precedes the new sub's EOSE.
	c.Subscribe(tests.RandomSubID(), map[string]interface{}{"kinds": []int{1}, "limit": 1})
	if reason := c.ExpectClosed(ids[1], 5*time.Second); !strings.Contains(reason, "too many subscriptions") {
		t.Fatalf("CLOSED reason %q", reason)
	}
}

// NIP-01 reserves CLOSED for relay-initiated ends; a client's own CLOSE —
// of a live or an unknown sub — gets no reply.
func TestCLOSE_NoReply(t *testing.T) {
	c := tests.NewTestClient(t)
	defer c.Close()

	subID := tests.RandomSubID()
	c.Subscribe(subID, map[string]interface{}{"kinds": []int{1}, "limit": 1})
	c.ExpectEOSE(subID, 5*time.Second)

	c.SendMessage([]interface{}{"CLOSE", subID})
	c.SendMessage([]interface{}{"CLOSE", "never-opened"})
	if msg, err := c.TryReadMessage(1500 * time.Millisecond); err == nil && msg != nil {
		t.Fatalf("unexpected reply to CLOSE: %v", msg)
	}
}
