package core

import (
	"crypto/rand"
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

// NIP44V3Encrypt encrypts plaintext to peerPubKey under NIP-44 v3, binding the
// event kind and scope. v3 is a draft — prefer NIP44Encrypt (v2) for interop.
func (es *EventSigner) NIP44V3Encrypt(peerPubKey string, kind uint32, scope, plaintext []byte) (string, error) {
	pub, err := xOnlyPubFromHex(peerPubKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return nip44EncryptV3(es.privateKey, pub, kind, scope, plaintext, nonce)
}

// NIP44V3Decrypt decrypts a NIP-44 v3 payload from peerPubKey, requiring it to
// match the expected kind + scope.
func (es *EventSigner) NIP44V3Decrypt(peerPubKey string, expectedKind uint32, expectedScope []byte, payload string) ([]byte, error) {
	pub, err := xOnlyPubFromHex(peerPubKey)
	if err != nil {
		return nil, err
	}
	return nip44DecryptV3(es.privateKey, pub, expectedKind, expectedScope, payload)
}

// xOnlyPubFromHex parses a 64-char hex x-only pubkey into a curve point.
func xOnlyPubFromHex(pubHex string) (*btcec.PublicKey, error) {
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, fmt.Errorf("nip44: invalid pubkey hex: %w", err)
	}
	return nip44ParseXOnlyPub(b)
}
