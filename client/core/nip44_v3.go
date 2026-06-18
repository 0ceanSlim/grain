package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"unicode/utf8"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/hkdf"
)

// NIP-44 v3 (https://github.com/nostr-land/nip44v3) — a draft proposal, not yet
// merged into nostr-protocol/nips. It improves on v2 by binding event `kind` and
// a custom `scope` into the authenticated data (cross-context replay protection
// + signer access control), supporting binary plaintext via a u32 length prefix,
// and salting the per-message HKDF with the nonce so ChaCha20 runs with a fixed
// zero nonce. Implemented against the proposal's official test-vectors.json.
//
// Encrypt still defaults to v2 elsewhere (deployed peers); v3 is opt-in here.

const nip44VersionV3 = 0x03

var nip44V3Salt = []byte("nip44-v3\x00")

// nip44PRKV3 is HKDF-Extract(salt = "nip44-v3\x00" || nonce, IKM = ecdh_x).
func nip44PRKV3(priv *btcec.PrivateKey, pub *btcec.PublicKey, nonce []byte) []byte {
	salt := make([]byte, 0, len(nip44V3Salt)+len(nonce))
	salt = append(salt, nip44V3Salt...)
	salt = append(salt, nonce...)
	return hkdf.Extract(sha256.New, ecdhX(priv, pub), salt)
}

// nip44MessageKeysV3 expands the PRK into the 32-byte encryption and MAC keys.
func nip44MessageKeysV3(priv *btcec.PrivateKey, pub *btcec.PublicKey, nonce []byte) (encKey, macKey []byte, err error) {
	prk := nip44PRKV3(priv, pub, nonce)
	if encKey, err = nip44HKDFExpand(prk, "encryption_key", 32); err != nil {
		return nil, nil, err
	}
	if macKey, err = nip44HKDFExpand(prk, "mac_key", 32); err != nil {
		return nil, nil, err
	}
	return encKey, macKey, nil
}

func nip44HKDFExpand(prk []byte, info string, l int) ([]byte, error) {
	out := make([]byte, l)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, []byte(info)), out); err != nil {
		return nil, err
	}
	return out, nil
}

// nip44TargetSizeV3 is the v3 padding target for a buffer of the given length
// (the spec's target_size, applied to the u32-prefixed plaintext).
func nip44TargetSizeV3(length int) int {
	const minSize, smallSub, largeSub, largeThresh = 32, 4, 8, 32768
	if length <= 0 {
		return minSize
	}
	nextPower := 1 << bits.Len(uint(length-1)) // smallest power of two >= length
	subdivs := smallSub
	if nextPower >= largeThresh {
		subdivs = largeSub
	}
	chunk := max(nextPower/subdivs, minSize)
	return chunk * ((length + chunk - 1) / chunk) // chunk * ceil(length/chunk)
}

// nip44ChaCha20V3 runs ChaCha20 with a 96-bit zero nonce (the key is unique per
// message, so the nonce can be fixed).
func nip44ChaCha20V3(key, data []byte) ([]byte, error) {
	c, err := chacha20.NewUnauthenticatedCipher(key, make([]byte, chacha20.NonceSize))
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out, nil
}

// nip44AuthDataV3 builds the MAC input: nonce || kind || len(scope) || scope || ct.
func nip44AuthDataV3(nonce []byte, kind uint32, scope, ciphertext []byte) []byte {
	ad := make([]byte, 0, len(nonce)+8+len(scope)+len(ciphertext))
	ad = append(ad, nonce...)
	ad = binary.BigEndian.AppendUint32(ad, kind)
	ad = binary.BigEndian.AppendUint32(ad, uint32(len(scope)))
	ad = append(ad, scope...)
	ad = append(ad, ciphertext...)
	return ad
}

// nip44EncryptV3 encrypts plaintext from priv to pub, binding kind + scope,
// with the given 32-byte nonce. Returns the base64 v3 payload.
func nip44EncryptV3(priv *btcec.PrivateKey, pub *btcec.PublicKey, kind uint32, scope, plaintext, nonce []byte) (string, error) {
	if len(nonce) != 32 {
		return "", errors.New("nip44: nonce must be 32 bytes")
	}
	if !utf8.Valid(scope) {
		return "", errors.New("nip44: scope is not valid UTF-8")
	}
	if int64(len(plaintext)) > (1<<31)-1 {
		return "", errors.New("nip44: plaintext too long")
	}
	encKey, macKey, err := nip44MessageKeysV3(priv, pub, nonce)
	if err != nil {
		return "", err
	}
	// prefixed = u32_be(len) || plaintext, then zero-padded to the target size.
	prefixed := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(plaintext)), uint32(len(plaintext)))
	prefixed = append(prefixed, plaintext...)
	padded := make([]byte, nip44TargetSizeV3(len(prefixed)))
	copy(padded, prefixed)

	ciphertext, err := nip44ChaCha20V3(encKey, padded)
	if err != nil {
		return "", err
	}
	mac := nip44HMACAad(macKey, nip44AuthDataV3(nonce, kind, scope, ciphertext), nil)

	// payload = 0x03 || nonce || mac || kind || len(scope) || scope || ct
	payload := make([]byte, 0, 1+32+32+8+len(scope)+len(ciphertext))
	payload = append(payload, nip44VersionV3)
	payload = append(payload, nonce...)
	payload = append(payload, mac...)
	payload = binary.BigEndian.AppendUint32(payload, kind)
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(scope)))
	payload = append(payload, scope...)
	payload = append(payload, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// nip44DecryptV3 decrypts a v3 payload, verifying it was encrypted for the
// expected kind + scope. Returns the binary plaintext.
func nip44DecryptV3(priv *btcec.PrivateKey, pub *btcec.PublicKey, expectedKind uint32, expectedScope []byte, payloadB64 string) ([]byte, error) {
	if len(payloadB64) == 0 || payloadB64[0] == '#' {
		return nil, errors.New("nip44: unsupported payload version")
	}
	data, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, errors.New("nip44: invalid base64 payload")
	}
	if len(data) < 77 {
		return nil, fmt.Errorf("nip44: payload too small (%d)", len(data))
	}
	if data[0] != nip44VersionV3 {
		return nil, fmt.Errorf("nip44: unknown version %d", data[0])
	}
	nonce := data[1:33]
	mac := data[33:65]
	kind := binary.BigEndian.Uint32(data[65:69])
	scopeLen := binary.BigEndian.Uint32(data[69:73])
	if int64(scopeLen) > int64(len(data)-73) {
		return nil, errors.New("nip44: scope length overflows payload")
	}
	scope := data[73 : 73+scopeLen]
	if !utf8.Valid(scope) {
		return nil, errors.New("nip44: scope is not valid UTF-8")
	}
	ciphertext := data[73+scopeLen:]
	if len(ciphertext) < 4 {
		return nil, errors.New("nip44: ciphertext too short")
	}
	if kind != expectedKind {
		return nil, fmt.Errorf("nip44: kind mismatch (got %d, want %d)", kind, expectedKind)
	}
	if !bytes.Equal(scope, expectedScope) {
		return nil, errors.New("nip44: scope mismatch")
	}
	encKey, macKey, err := nip44MessageKeysV3(priv, pub, nonce)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(nip44HMACAad(macKey, nip44AuthDataV3(nonce, kind, scope, ciphertext), nil), mac) {
		return nil, errors.New("nip44: invalid MAC")
	}
	padded, err := nip44ChaCha20V3(encKey, ciphertext)
	if err != nil {
		return nil, err
	}
	plLen := binary.BigEndian.Uint32(padded[0:4])
	if int64(plLen)+4 > int64(len(padded)) || plLen > (1<<31)-1 {
		return nil, errors.New("nip44: invalid plaintext length")
	}
	if !nip44AllZero(padded[4+plLen:]) {
		return nil, errors.New("nip44: non-zero padding")
	}
	return padded[4 : 4+plLen], nil
}

// nip44AllZero reports whether b is all zero bytes, in constant time.
func nip44AllZero(b []byte) bool {
	var v byte
	for _, x := range b {
		v |= x
	}
	return v == 0
}
