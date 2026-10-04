package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Events whose content or tags held &, < or > used to be acked OK and then
// silently dropped: grain handed nostrdb json.Marshal output, which escapes
// them as \uXXXX, and nostrdb's parser rejects \u escapes. Found on a
// production relay where no stored note contained any of the three.

func TestEventStrings_HTMLCharsAreStored(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	cases := []struct {
		kind    int
		content string
		tags    [][]string
	}{
		{1, "a & b < c > d", nil},
		{1, "see https://x.com/p?a=1&b=2", [][]string{{"r", "https://x.com/p?a=1&b=2"}}},
		{1, "a &amp; b", [][]string{{"t", "<tag>"}}},
		{0, `{"about":"Find me > #squatpushrepeat"}`, nil},
		{30023, "<h1>title</h1>", [][]string{{"d", "a&b"}}},
	}
	for _, tc := range cases {
		evt := kp.SignEvent(tc.kind, tc.content, tc.tags)
		client.SendEvent(evt)
		if ok, reason := client.ExpectOK(evt.ID, 5*time.Second); !ok {
			t.Fatalf("kind %d %q rejected: %s", tc.kind, tc.content, reason)
		}
		if !client.AwaitCommit(evt.ID, 5*time.Second) {
			t.Fatalf("kind %d %q acked OK but never stored", tc.kind, tc.content)
		}

		subID := tests.RandomSubID()
		client.Subscribe(subID, map[string]interface{}{"ids": []string{evt.ID}})
		events := client.ExpectEOSE(subID, 5*time.Second)
		if len(events) != 1 {
			t.Fatalf("kind %d %q: got %d events, want 1", tc.kind, tc.content, len(events))
		}
		if got := events[0]["content"]; got != tc.content {
			t.Errorf("content: got %q, want %q", got, tc.content)
		}
	}
}

// Frames must carry &, < and > raw: clients that ingest relay messages with
// nostrdb (Damus, Notedeck) drop anything holding a \u escape.
func TestEventStrings_SentUnescaped(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()

	content := "wire a & b <c>"
	evt := kp.SignEvent(1, content, nil)
	client.SendEvent(evt)
	if ok, reason := client.ExpectOK(evt.ID, 5*time.Second); !ok {
		t.Fatalf("rejected: %s", reason)
	}
	if !client.AwaitCommit(evt.ID, 5*time.Second) {
		t.Fatal("acked OK but never stored")
	}

	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{"ids": []string{evt.ID}})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := string(client.ReadMessageRaw(time.Until(deadline)))
		if strings.Contains(raw, `"EVENT"`) && strings.Contains(raw, subID) {
			if !strings.Contains(raw, content) {
				t.Fatalf("EVENT frame escapes the content: %s", raw)
			}
			return
		}
	}
	t.Fatal("no EVENT frame for the subscription")
}

// A replaceable used to lose both versions: the old one was deleted before
// the new one went to the ingester, which then dropped it.
func TestReplaceableProfile_HTMLCharsReplaceOld(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClient(t)
	defer client.Close()
	now := time.Now().Unix()

	old := kp.SignEventAt(0, `{"name":"old"}`, nil, now-60)
	client.SendEvent(old)
	if ok, reason := client.ExpectOK(old.ID, 5*time.Second); !ok {
		t.Fatalf("old profile rejected: %s", reason)
	}
	if !client.AwaitCommit(old.ID, 5*time.Second) {
		t.Fatal("old profile not committed")
	}

	newer := kp.SignEventAt(0, `{"about":"Shameless #Sidecar & #Clave advocate"}`, nil, now)
	client.SendEvent(newer)
	if ok, reason := client.ExpectOK(newer.ID, 5*time.Second); !ok {
		t.Fatalf("new profile rejected: %s", reason)
	}
	if !client.AwaitCommit(newer.ID, 5*time.Second) {
		t.Fatal("new profile acked OK but never stored")
	}

	subID := tests.RandomSubID()
	client.Subscribe(subID, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{0},
	})
	events := client.ExpectEOSE(subID, 5*time.Second)
	if len(events) != 1 || events[0]["id"] != newer.ID {
		t.Fatalf("want only the new profile %s, got %v", newer.ID, events)
	}
}

// A tag spelled like an event field ("pubkey") made nostrdb rewrite that
// field and refuse the event. Seen live on kind 4454.
func TestEventStrings_FieldNamedTagStored(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	evt := kp.SignEvent(4454, "", [][]string{{"pubkey", strings.Repeat("ab", 32)}, {"t", "content"}})
	c.SendEvent(evt)
	if ok, reason := c.ExpectOK(evt.ID, 5*time.Second); !ok {
		t.Fatalf("rejected: %q", reason)
	}
	if !c.AwaitCommit(evt.ID, 5*time.Second) {
		t.Fatal("acked OK but never stored")
	}
}
