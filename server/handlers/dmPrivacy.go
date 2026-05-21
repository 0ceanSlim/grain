package handlers

import (
	"github.com/0ceanslim/grain/config"
	nostr "github.com/0ceanslim/grain/server/types"
)

// NIP-17 gift-wrap metadata privacy (#73). A protected-kind event (kind:1059)
// is served only to a client AUTHed as a pubkey p-tagged on it. These helpers
// are shared by the historical REQ path, the COUNT path, and the live-broadcast
// path (server/client.go), so the gate is identical everywhere.

// CanServeProtectedEvent reports whether evt may be served to a client that has
// AUTHed as authedPubkey ("" if unauthenticated). Non-protected events always
// pass; a protected event passes only when the authed pubkey appears in one of
// its p tags.
func CanServeProtectedEvent(evt nostr.Event, authedPubkey string) bool {
	if !config.IsDMProtectedKind(evt.Kind) {
		return true
	}
	if authedPubkey == "" {
		return false
	}
	return eventHasPTag(evt, authedPubkey)
}

// FilterRequestsProtectedKind reports whether the filter explicitly asks for a
// protected kind. Used to decide when an unauthenticated REQ/COUNT should be
// told to AUTH (vs. a broad filter where we just silently omit the events).
func FilterRequestsProtectedKind(f nostr.Filter) bool {
	for _, k := range f.Kinds {
		if config.IsDMProtectedKind(k) {
			return true
		}
	}
	return false
}

// eventHasPTag reports whether evt carries a `p` tag whose value equals pubkey.
func eventHasPTag(evt nostr.Event, pubkey string) bool {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] == pubkey {
			return true
		}
	}
	return false
}
