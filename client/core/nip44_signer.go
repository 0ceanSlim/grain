package core

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

// EventSigner satisfies Encrypter: NIP-44 encrypt/decrypt with the held private
// key. grain's own web client encrypts in the browser (NIP-07/NIP-46); these
// methods are for downstream Go consumers and server-side-key sessions.

// NIP44Encrypt encrypts plaintext to peerPubKey using NIP-44 v2. peerPubKey is
// 64-char hex (x-only); for NIP-51 private lists (encrypted to self) pass the
// signer's own pubkey.
func (es *EventSigner) NIP44Encrypt(peerPubKey, plaintext string) (string, error) {
	pub, err := xOnlyPubFromHex(peerPubKey)
	if err != nil {
		return "", err
	}
	return nip44Encrypt(es.privateKey, pub, plaintext)
}

// NIP44Decrypt decrypts a base64 NIP-44 payload from peerPubKey (any supported
// version).
func (es *EventSigner) NIP44Decrypt(peerPubKey, payload string) (string, error) {
	pub, err := xOnlyPubFromHex(peerPubKey)
	if err != nil {
		return "", err
	}
	return nip44Decrypt(es.privateKey, pub, payload)
}

// xOnlyPubFromHex parses a 64-char hex x-only pubkey into a curve point.
func xOnlyPubFromHex(pubHex string) (*btcec.PublicKey, error) {
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, fmt.Errorf("nip44: invalid pubkey hex: %w", err)
	}
	return nip44ParseXOnlyPub(b)
}
