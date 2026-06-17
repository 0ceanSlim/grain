package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/hkdf"
)

// NIP-44 versioned payload encryption (https://github.com/nostr-protocol/nips/blob/master/44.md).
//
// v2 — the deployed standard — is implemented here: secp256k1 ECDH → HKDF-SHA256
// conversation key, per-message HKDF-expanded ChaCha20 + HMAC-SHA256, with the
// spec's power-of-two padding. Validated against the official test vectors in
// nip44_test.go. v3 (the nostr-land proposal) layers on top in nip44_v3.go.
//
// grain rolls its own nostr layer, so this is a from-scratch implementation
// rather than a library dependency; correctness rides entirely on the vectors.

const nip44VersionV2 = 0x02

// nip44ConversationKeyV2 derives the v2 conversation key shared between a private
// key and a peer's public key: hkdf_extract(IKM = ecdh_x, salt = "nip44-v2").
func nip44ConversationKeyV2(priv *btcec.PrivateKey, pub *btcec.PublicKey) []byte {
	shared := ecdhX(priv, pub)
	return hkdf.Extract(sha256.New, shared, []byte("nip44-v2"))
}

// ecdhX returns the 32-byte big-endian X coordinate of priv*pub on secp256k1 —
// the raw shared coordinate NIP-44 feeds into HKDF (not the SHA-256-hashed
// shared secret that btcec.GenerateSharedSecret would give).
func ecdhX(priv *btcec.PrivateKey, pub *btcec.PublicKey) []byte {
	var point, result secp256k1.JacobianPoint
	pub.AsJacobian(&point)
	secp256k1.ScalarMultNonConst(&priv.Key, &point, &result)
	result.ToAffine()
	xb := result.X.Bytes() // *[32]byte
	return xb[:]
}

// nip44MessageKeysV2 expands the conversation key + 32-byte nonce into the
// per-message ChaCha20 key (32), ChaCha20 nonce (12), and HMAC key (32).
func nip44MessageKeysV2(convKey, nonce []byte) (chachaKey, chachaNonce, hmacKey []byte, err error) {
	r := hkdf.Expand(sha256.New, convKey, nonce)
	out := make([]byte, 76)
	if _, err = io.ReadFull(r, out); err != nil {
		return nil, nil, nil, err
	}
	return out[0:32], out[32:44], out[44:76], nil
}

// nip44PadLen is the spec's calc_padded_len: pad the plaintext up to a
// power-of-two-derived chunk so ciphertext length leaks as little as possible.
func nip44PadLen(unpadded int) int {
	if unpadded <= 32 {
		return 32
	}
	nextPower := 1 << bits.Len(uint(unpadded-1)) // = 2^(floor(log2(n-1))+1), exact
	chunk := 32
	if nextPower > 256 {
		chunk = nextPower / 8
	}
	return chunk * ((unpadded-1)/chunk + 1)
}

// nip44Pad frames plaintext as: u16_be(len) || plaintext || zero padding.
func nip44Pad(plaintext []byte) ([]byte, error) {
	ul := len(plaintext)
	if ul < 1 || ul > 65535 {
		return nil, fmt.Errorf("nip44: plaintext length %d out of range [1,65535]", ul)
	}
	out := make([]byte, 2+nip44PadLen(ul))
	binary.BigEndian.PutUint16(out[0:2], uint16(ul))
	copy(out[2:], plaintext)
	return out, nil
}

// nip44Unpad recovers and validates the plaintext from a padded buffer.
func nip44Unpad(padded []byte) ([]byte, error) {
	if len(padded) < 2 {
		return nil, errors.New("nip44: padding too short")
	}
	ul := int(binary.BigEndian.Uint16(padded[0:2]))
	if ul < 1 || 2+ul > len(padded) || len(padded) != 2+nip44PadLen(ul) {
		return nil, errors.New("nip44: invalid padding")
	}
	return padded[2 : 2+ul], nil
}

// nip44EncryptV2 encrypts plaintext under a conversation key with the given
// 32-byte nonce, returning the base64 payload (version || nonce || ct || mac).
func nip44EncryptV2(plaintext string, convKey, nonce []byte) (string, error) {
	if len(convKey) != 32 {
		return "", errors.New("nip44: conversation key must be 32 bytes")
	}
	if len(nonce) != 32 {
		return "", errors.New("nip44: nonce must be 32 bytes")
	}
	chachaKey, chachaNonce, hmacKey, err := nip44MessageKeysV2(convKey, nonce)
	if err != nil {
		return "", err
	}
	padded, err := nip44Pad([]byte(plaintext))
	if err != nil {
		return "", err
	}
	cipher, err := chacha20.NewUnauthenticatedCipher(chachaKey, chachaNonce)
	if err != nil {
		return "", err
	}
	ciphertext := make([]byte, len(padded))
	cipher.XORKeyStream(ciphertext, padded)

	mac := nip44HMACAad(hmacKey, nonce, ciphertext)

	payload := make([]byte, 0, 1+len(nonce)+len(ciphertext)+len(mac))
	payload = append(payload, nip44VersionV2)
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)
	payload = append(payload, mac...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// nip44DecryptV2 decrypts a v2 base64 payload under a conversation key.
func nip44DecryptV2(payloadB64 string, convKey []byte) (string, error) {
	if len(convKey) != 32 {
		return "", errors.New("nip44: conversation key must be 32 bytes")
	}
	if len(payloadB64) == 0 || payloadB64[0] == '#' {
		return "", errors.New("nip44: unsupported payload version")
	}
	data, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", errors.New("nip44: invalid base64 payload")
	}
	// version(1) + nonce(32) + min ciphertext(34) + mac(32)
	if len(data) < 99 || len(data) > 65603 {
		return "", fmt.Errorf("nip44: invalid payload size %d", len(data))
	}
	if data[0] != nip44VersionV2 {
		return "", fmt.Errorf("nip44: unknown version %d", data[0])
	}
	nonce := data[1:33]
	ciphertext := data[33 : len(data)-32]
	mac := data[len(data)-32:]

	chachaKey, chachaNonce, hmacKey, err := nip44MessageKeysV2(convKey, nonce)
	if err != nil {
		return "", err
	}
	if !hmac.Equal(nip44HMACAad(hmacKey, nonce, ciphertext), mac) {
		return "", errors.New("nip44: invalid MAC")
	}
	cipher, err := chacha20.NewUnauthenticatedCipher(chachaKey, chachaNonce)
	if err != nil {
		return "", err
	}
	padded := make([]byte, len(ciphertext))
	cipher.XORKeyStream(padded, ciphertext)
	unpadded, err := nip44Unpad(padded)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

// nip44PayloadVersion peeks the version byte of a base64 payload without
// decrypting, so decrypt can route to the right scheme (v2 and v3 derive their
// conversation keys differently).
func nip44PayloadVersion(payload string) (byte, error) {
	if len(payload) == 0 || payload[0] == '#' {
		return 0, errors.New("nip44: unsupported payload version")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(data) < 1 {
		return 0, errors.New("nip44: invalid payload")
	}
	return data[0], nil
}

// nip44Encrypt encrypts plaintext from priv to peerPub using the default scheme
// (v2 — the deployed standard) with a fresh random nonce. v3 stays opt-in until
// the proposal has deployed peers.
func nip44Encrypt(priv *btcec.PrivateKey, peerPub *btcec.PublicKey, plaintext string) (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return nip44EncryptV2(plaintext, nip44ConversationKeyV2(priv, peerPub), nonce)
}

// nip44Decrypt routes a payload to the matching version's decryptor.
func nip44Decrypt(priv *btcec.PrivateKey, peerPub *btcec.PublicKey, payload string) (string, error) {
	ver, err := nip44PayloadVersion(payload)
	if err != nil {
		return "", err
	}
	switch ver {
	case nip44VersionV2:
		return nip44DecryptV2(payload, nip44ConversationKeyV2(priv, peerPub))
	case nip44VersionV3:
		// v3 authenticates an event kind + scope the generic seam doesn't carry.
		return "", errors.New("nip44: v3 payload requires kind+scope context")
	default:
		return "", fmt.Errorf("nip44: unsupported version %d", ver)
	}
}

// nip44HMACAad is HMAC-SHA256 over (aad || message) — NIP-44 authenticates the
// nonce as associated data alongside the ciphertext.
func nip44HMACAad(key, aad, message []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(aad)
	h.Write(message)
	return h.Sum(nil)
}

// nip44ParseXOnlyPub lifts a 32-byte x-only (BIP340) public key to a full point
// with even Y, as NIP-44 ECDH expects.
func nip44ParseXOnlyPub(xOnly []byte) (*btcec.PublicKey, error) {
	if len(xOnly) != 32 {
		return nil, fmt.Errorf("nip44: x-only pubkey must be 32 bytes, got %d", len(xOnly))
	}
	return schnorr.ParsePubKey(xOnly)
}

// nip44ParsePriv validates and parses a 32-byte secret key, rejecting keys that
// are zero or >= the curve order (which btcec.PrivKeyFromBytes would silently
// reduce instead of rejecting).
func nip44ParsePriv(secKey []byte) (*btcec.PrivateKey, error) {
	if len(secKey) != 32 {
		return nil, fmt.Errorf("nip44: secret key must be 32 bytes, got %d", len(secKey))
	}
	var s secp256k1.ModNScalar
	if overflow := s.SetByteSlice(secKey); overflow {
		return nil, errors.New("nip44: secret key >= curve order")
	}
	if s.IsZero() {
		return nil, errors.New("nip44: secret key is zero")
	}
	return secp256k1.NewPrivateKey(&s), nil
}
