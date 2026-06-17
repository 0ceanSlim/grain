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

// Encrypter is an optional capability a Signer may also implement: NIP-44
// payload encryption between the signer's key and a peer pubkey, used for
// NIP-51 private list content, NIP-17 DMs, and similar. It mirrors the
// nip44Encrypt/nip44Decrypt methods a NIP-07/NIP-46 browser signer exposes, so
// the same calling code works whether the key lives in the browser or in a
// downstream Go consumer's [EventSigner].
//
// peerPubKey is 64-char lowercase hex (x-only). For self-encryption (NIP-51
// lists are encrypted to the author themselves) pass the signer's own pubkey.
type Encrypter interface {
	// NIP44Encrypt encrypts plaintext to peerPubKey, returning a base64 NIP-44
	// payload (v2, the deployed standard).
	NIP44Encrypt(peerPubKey, plaintext string) (string, error)
	// NIP44Decrypt decrypts a base64 NIP-44 payload from peerPubKey, accepting
	// any supported version.
	NIP44Decrypt(peerPubKey, payload string) (string, error)
}

// The built-in local-key signer satisfies both seams.
var (
	_ Signer    = (*EventSigner)(nil)
	_ Encrypter = (*EventSigner)(nil)
)
