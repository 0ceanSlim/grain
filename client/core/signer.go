package core

import nostr "github.com/0ceanslim/grain/server/types"

// Signer produces signatures for Nostr events on behalf of one pubkey. A library
// consumer supplies a Signer to publish — a local key via [EventSigner], or their
// own implementation (NIP-46 remote signer, hardware, HSM). Read-only callers
// supply none.
//
// grain's own web client signs in the browser with the user's NIP-07 / NIP-46
// signer, so grain's server side carries no Signer; this seam exists for
// downstream Go consumers building a client on the library.
type Signer interface {
	// PublicKey returns the signer's public key as 64-char lowercase hex.
	PublicKey() string
	// SignEvent fills the event's PubKey, ID, and Sig fields in place: PubKey is
	// set to PublicKey(), and Sig is a Schnorr signature over the NIP-01 id
	// computed from the serialized event.
	SignEvent(event *nostr.Event) error
}

// The built-in local-key signer satisfies the seam.
var _ Signer = (*EventSigner)(nil)
