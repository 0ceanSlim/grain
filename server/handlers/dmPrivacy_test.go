package handlers

import (
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

const (
	recipientPK = "1111111111111111111111111111111111111111111111111111111111111111"
	strangerPK  = "2222222222222222222222222222222222222222222222222222222222222222"
)

func giftWrap(recipient string) nostr.Event {
	return nostr.Event{Kind: 1059, Tags: [][]string{{"p", recipient}}}
}

func TestCanServeProtectedEvent(t *testing.T) {
	wrap := giftWrap(recipientPK)

	// Non-protected events are always servable, even unauthenticated.
	if !CanServeProtectedEvent(nostr.Event{Kind: 1}, "") {
		t.Error("non-protected kind should be servable unauthenticated")
	}
	// Gift wrap: unauthenticated callers get nothing.
	if CanServeProtectedEvent(wrap, "") {
		t.Error("gift wrap must not be served to an unauthenticated client")
	}
	// Gift wrap: a stranger (authed as someone else) gets nothing.
	if CanServeProtectedEvent(wrap, strangerPK) {
		t.Error("gift wrap must not be served to a non-recipient")
	}
	// Gift wrap: the p-tagged recipient gets it.
	if !CanServeProtectedEvent(wrap, recipientPK) {
		t.Error("gift wrap must be served to its p-tagged recipient")
	}
}

func TestFilterRequestsProtectedKind(t *testing.T) {
	if !FilterRequestsProtectedKind(nostr.Filter{Kinds: []int{1, 1059}}) {
		t.Error("filter listing 1059 should be flagged")
	}
	if FilterRequestsProtectedKind(nostr.Filter{Kinds: []int{1, 7}}) {
		t.Error("filter without protected kinds should not be flagged")
	}
	// A no-kinds filter doesn't *explicitly* request a protected kind — the
	// post-query gate handles silent omission for broad filters.
	if FilterRequestsProtectedKind(nostr.Filter{}) {
		t.Error("empty-kinds filter should not be flagged as explicitly requesting")
	}
}
