package core

import (
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// newClientWithStubDirectory builds a Client whose relay directory returns
// canned UserRelays, so routing can be tested without any network.
func newClientWithStubDirectory(relays map[string]*UserRelays) *Client {
	c := NewClient(DefaultConfig())
	c.directory = newRelayDirectory(time.Hour, time.Minute, func(pk string) *UserRelays {
		if ur, ok := relays[pk]; ok {
			return ur
		}
		return nil
	})
	return c
}

func assertNotContains(t *testing.T, urls []string, unwanted string) {
	t.Helper()
	for _, u := range urls {
		if u == unwanted {
			t.Fatalf("did not expect %q in %v", unwanted, urls)
		}
	}
}

func TestPTaggedPubkeys(t *testing.T) {
	ev := &nostr.Event{Tags: [][]string{{"p", "a"}, {"e", "x"}, {"p", "b"}, {"p", "a"}}}
	got := pTaggedPubkeys(ev)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("pTaggedPubkeys = %v, want [a b]", got)
	}
}

func TestRouteFetchUsesOutboxThenIndex(t *testing.T) {
	c := newClientWithStubDirectory(map[string]*UserRelays{
		"author": {Outbox: []string{"wss://a-out"}},
	})
	if got := c.RouteFetch("author"); len(got) != 1 || got[0] != "wss://a-out" {
		t.Fatalf("RouteFetch(author) = %v, want [wss://a-out]", got)
	}
	// Unknown user → index-relay fallback.
	if got := c.RouteFetch("nobody"); len(got) == 0 || got[0] != c.config.IndexRelays[0] {
		t.Fatalf("RouteFetch(nobody) should fall back to index relays, got %v", got)
	}
}

// The crux of the outbox model: a reply goes to MY outbox AND the recipient's
// inbox — so it both reaches my audience and lands where they read.
func TestRoutePublishReplyHitsOwnOutboxAndTargetInbox(t *testing.T) {
	c := newClientWithStubDirectory(map[string]*UserRelays{
		"me":  {Outbox: []string{"wss://my-out"}},
		"you": {Inbox: []string{"wss://your-in"}, Outbox: []string{"wss://your-out"}},
	})
	reply := &nostr.Event{PubKey: "me", Kind: 1, Tags: [][]string{{"p", "you"}}}
	got := c.RoutePublish(reply)
	assertHasRelay(t, got, "wss://my-out")      // my outbox
	assertHasRelay(t, got, "wss://your-in")     // recipient's inbox
	assertNotContains(t, got, "wss://your-out") // not their outbox
}

func TestRoutePublishGiftWrapHitsDMInbox(t *testing.T) {
	c := newClientWithStubDirectory(map[string]*UserRelays{
		"me":  {Outbox: []string{"wss://my-out"}},
		"you": {Inbox: []string{"wss://your-in"}, DMInbox: []string{"wss://your-dm"}},
	})
	wrap := &nostr.Event{PubKey: "me", Kind: 1059, Tags: [][]string{{"p", "you"}}}
	got := c.RoutePublish(wrap)
	assertHasRelay(t, got, "wss://your-dm")    // NIP-17 → DM inbox
	assertNotContains(t, got, "wss://your-in") // not the public inbox
}

func TestRoutePublishFallsBackToIndex(t *testing.T) {
	c := newClientWithStubDirectory(map[string]*UserRelays{})
	ev := &nostr.Event{PubKey: "me", Kind: 1}
	if got := c.RoutePublish(ev); len(got) == 0 || got[0] != c.config.IndexRelays[0] {
		t.Fatalf("RoutePublish with nothing resolved should fall back to index, got %v", got)
	}
}
