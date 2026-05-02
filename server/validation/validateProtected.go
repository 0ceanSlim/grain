package validation

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// IsProtectedEvent reports whether the event carries a NIP-70
// protection marker — a single-element `["-"]` tag. Per NIP-70 such
// events MUST only be accepted from an authenticated connection whose
// pubkey matches the event author.
func IsProtectedEvent(evt nostr.Event) bool {
	for _, tag := range evt.Tags {
		if len(tag) >= 1 && tag[0] == "-" {
			return true
		}
	}
	return false
}
