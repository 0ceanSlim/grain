package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

type nip44V3EncDec struct {
	Secret1       string `json:"secret1"`
	Secret2       string `json:"secret2"`
	Nonce         string `json:"nonce"`
	Kind          uint32 `json:"kind"`
	ScopeHex      string `json:"scope_hex"`
	PRK           string `json:"prk"`
	EncryptionKey string `json:"encryption_key"`
	MacKey        string `json:"mac_key"`
	PlaintextHex  string `json:"plaintext_hex"`
	Ciphertext    string `json:"ciphertext"`
	Note          string `json:"note"`
}

type nip44V3VectorFile struct {
	EncryptDecrypt     []nip44V3EncDec `json:"encrypt_decrypt"`
	DecryptOnly        []nip44V3EncDec `json:"decrypt_only"`
	LongEncryptDecrypt []struct {
		Secret1          string `json:"secret1"`
		Secret2          string `json:"secret2"`
		Nonce            string `json:"nonce"`
		Kind             uint32 `json:"kind"`
		ScopeHex         string `json:"scope_hex"`
		PatternHex       string `json:"pattern_hex"`
		Repeat           int    `json:"repeat"`
		CiphertextSha256 string `json:"ciphertext_sha256"`
	} `json:"long_encrypt_decrypt"`
	PaddedLength      [][2]int `json:"padded_length"`
	InvalidDecryption []struct {
		Secret     string `json:"secret"`
		Public     string `json:"public"`
		Kind       uint32 `json:"kind"`
		ScopeHex   string `json:"scope_hex"`
		Ciphertext string `json:"ciphertext"`
		Why        string `json:"why"`
	} `json:"invalid_decryption"`
}

func loadNIP44V3Vectors(t *testing.T) *nip44V3VectorFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/nip44v3.vectors.json")
	if err != nil {
		t.Fatalf("read v3 vectors: %v", err)
	}
	var v nip44V3VectorFile
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse v3 vectors: %v", err)
	}
	return &v
}

// nip44V3KeyPair returns the encrypting party's private key and the peer's
// public key from two secret-key hexes.
func nip44V3KeyPair(t *testing.T, sec1, sec2 string) (priv *btcec.PrivateKey, pub *btcec.PublicKey) {
	t.Helper()
	p1, err := nip44ParsePriv(mustHex(t, sec1))
	if err != nil {
		t.Fatalf("parse secret1: %v", err)
	}
	p2, err := nip44ParsePriv(mustHex(t, sec2))
	if err != nil {
		t.Fatalf("parse secret2: %v", err)
	}
	return p1, p2.PubKey()
}

func TestNIP44V3PaddedLength(t *testing.T) {
	v := loadNIP44V3Vectors(t)
	for _, pair := range v.PaddedLength {
		if got := nip44TargetSizeV3(pair[0]); got != pair[1] {
			t.Errorf("targetSize(%d) = %d, want %d", pair[0], got, pair[1])
		}
	}
}

func TestNIP44V3EncryptDecrypt(t *testing.T) {
	v := loadNIP44V3Vectors(t)
	for i, c := range v.EncryptDecrypt {
		priv, pub := nip44V3KeyPair(t, c.Secret1, c.Secret2)
		nonce := mustHex(t, c.Nonce)
		scope := mustHex(t, c.ScopeHex)
		plaintext := mustHex(t, c.PlaintextHex)

		// Validate every key-derivation step the vector exposes.
		if got := hex.EncodeToString(nip44PRKV3(priv, pub, nonce)); got != c.PRK {
			t.Errorf("case %d: prk = %s, want %s", i, got, c.PRK)
		}
		ek, mk, err := nip44MessageKeysV3(priv, pub, nonce)
		if err != nil {
			t.Fatalf("case %d: message keys: %v", i, err)
		}
		if got := hex.EncodeToString(ek); got != c.EncryptionKey {
			t.Errorf("case %d: encryption_key = %s, want %s", i, got, c.EncryptionKey)
		}
		if got := hex.EncodeToString(mk); got != c.MacKey {
			t.Errorf("case %d: mac_key = %s, want %s", i, got, c.MacKey)
		}

		got, err := nip44EncryptV3(priv, pub, c.Kind, scope, plaintext, nonce)
		if err != nil {
			t.Fatalf("case %d: encrypt: %v", i, err)
		}
		if got != c.Ciphertext {
			t.Errorf("case %d: ciphertext = %s, want %s", i, got, c.Ciphertext)
		}
		out, err := nip44DecryptV3(priv, pub, c.Kind, scope, c.Ciphertext)
		if err != nil {
			t.Fatalf("case %d: decrypt: %v", i, err)
		}
		if !bytes.Equal(out, plaintext) {
			t.Errorf("case %d: plaintext = %x, want %x", i, out, plaintext)
		}
	}
}

func TestNIP44V3DecryptOnly(t *testing.T) {
	v := loadNIP44V3Vectors(t)
	for i, c := range v.DecryptOnly {
		priv, pub := nip44V3KeyPair(t, c.Secret1, c.Secret2)
		out, err := nip44DecryptV3(priv, pub, c.Kind, mustHex(t, c.ScopeHex), c.Ciphertext)
		if err != nil {
			t.Fatalf("case %d (%s): decrypt: %v", i, c.Note, err)
		}
		if want := mustHex(t, c.PlaintextHex); !bytes.Equal(out, want) {
			t.Errorf("case %d: plaintext = %x, want %x", i, out, want)
		}
	}
}

func TestNIP44V3LongMessages(t *testing.T) {
	v := loadNIP44V3Vectors(t)
	for i, c := range v.LongEncryptDecrypt {
		priv, pub := nip44V3KeyPair(t, c.Secret1, c.Secret2)
		plaintext := bytes.Repeat(mustHex(t, c.PatternHex), c.Repeat)
		scope := mustHex(t, c.ScopeHex)
		payload, err := nip44EncryptV3(priv, pub, c.Kind, scope, plaintext, mustHex(t, c.Nonce))
		if err != nil {
			t.Fatalf("case %d: encrypt: %v", i, err)
		}
		if got := sha256Hex(payload); got != c.CiphertextSha256 {
			t.Errorf("case %d: ciphertext sha256 = %s, want %s", i, got, c.CiphertextSha256)
		}
		out, err := nip44DecryptV3(priv, pub, c.Kind, scope, payload)
		if err != nil {
			t.Fatalf("case %d: decrypt: %v", i, err)
		}
		if !bytes.Equal(out, plaintext) {
			t.Errorf("case %d: round-trip mismatch", i)
		}
	}
}

// TestNIP44V3EventSignerRoundTrip exercises the v3 seam and its defining
// feature: a payload only decrypts under the kind + scope it was bound to.
func TestNIP44V3EventSignerRoundTrip(t *testing.T) {
	alice, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	const kind = uint32(10013)
	scope := []byte("spec.nostr.land/relay-lists")
	plaintext := []byte("private relay list payload \x00\x01\x02") // binary-safe

	ct, err := alice.NIP44V3Encrypt(bob.PublicKey(), kind, scope, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if got, err := bob.NIP44V3Decrypt(alice.PublicKey(), kind, scope, ct); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip: got %x err %v", got, err)
	}
	// Context binding: wrong kind or scope must fail.
	if _, err := bob.NIP44V3Decrypt(alice.PublicKey(), kind+1, scope, ct); err == nil {
		t.Error("expected wrong-kind decrypt to fail")
	}
	if _, err := bob.NIP44V3Decrypt(alice.PublicKey(), kind, []byte("other-scope"), ct); err == nil {
		t.Error("expected wrong-scope decrypt to fail")
	}
	// The generic (v2) decrypt path must refuse a v3 payload.
	if _, err := bob.NIP44Decrypt(alice.PublicKey(), ct); err == nil {
		t.Error("expected v2 decrypt path to reject a v3 payload")
	}
}

func TestNIP44V3InvalidDecryption(t *testing.T) {
	v := loadNIP44V3Vectors(t)
	for i, c := range v.InvalidDecryption {
		priv, err := nip44ParsePriv(mustHex(t, c.Secret))
		if err != nil {
			continue // an unusable key is itself a rejection
		}
		pub, err := nip44ParseXOnlyPub(mustHex(t, c.Public))
		if err != nil {
			continue
		}
		if _, err := nip44DecryptV3(priv, pub, c.Kind, mustHex(t, c.ScopeHex), c.Ciphertext); err == nil {
			t.Errorf("case %d (%s): expected decrypt to fail, it succeeded", i, c.Why)
		}
	}
}
