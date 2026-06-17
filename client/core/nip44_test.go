package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

// nip44VectorFile mirrors the official NIP-44 test vectors
// (testdata/nip44.vectors.json from github.com/paulmillr/nip44).
type nip44VectorFile struct {
	V2 struct {
		Valid struct {
			GetConversationKey []struct {
				Sec1            string `json:"sec1"`
				Pub2            string `json:"pub2"`
				ConversationKey string `json:"conversation_key"`
			} `json:"get_conversation_key"`
			GetMessageKeys struct {
				ConversationKey string `json:"conversation_key"`
				Keys            []struct {
					Nonce       string `json:"nonce"`
					ChachaKey   string `json:"chacha_key"`
					ChachaNonce string `json:"chacha_nonce"`
					HmacKey     string `json:"hmac_key"`
				} `json:"keys"`
			} `json:"get_message_keys"`
			CalcPaddedLen         [][2]int `json:"calc_padded_len"`
			EncryptDecrypt        []nip44EncDecVec
			EncryptDecryptLongMsg []struct {
				ConversationKey string `json:"conversation_key"`
				Nonce           string `json:"nonce"`
				Pattern         string `json:"pattern"`
				Repeat          int    `json:"repeat"`
				PlaintextSha256 string `json:"plaintext_sha256"`
				PayloadSha256   string `json:"payload_sha256"`
			} `json:"encrypt_decrypt_long_msg"`
		} `json:"valid"`
		Invalid struct {
			GetConversationKey []struct {
				Sec1 string `json:"sec1"`
				Pub2 string `json:"pub2"`
				Note string `json:"note"`
			} `json:"get_conversation_key"`
			Decrypt []struct {
				ConversationKey string `json:"conversation_key"`
				Payload         string `json:"payload"`
				Note            string `json:"note"`
			} `json:"decrypt"`
		} `json:"invalid"`
	} `json:"v2"`
}

type nip44EncDecVec struct {
	Sec1            string `json:"sec1"`
	Sec2            string `json:"sec2"`
	ConversationKey string `json:"conversation_key"`
	Nonce           string `json:"nonce"`
	Plaintext       string `json:"plaintext"`
	Payload         string `json:"payload"`
}

func loadNIP44Vectors(t *testing.T) *nip44VectorFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/nip44.vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var v nip44VectorFile
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	return &v
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestNIP44V2GetConversationKey(t *testing.T) {
	v := loadNIP44Vectors(t)
	for i, c := range v.V2.Valid.GetConversationKey {
		priv, err := nip44ParsePriv(mustHex(t, c.Sec1))
		if err != nil {
			t.Fatalf("case %d: parse sec1: %v", i, err)
		}
		pub, err := nip44ParseXOnlyPub(mustHex(t, c.Pub2))
		if err != nil {
			t.Fatalf("case %d: parse pub2: %v", i, err)
		}
		got := hex.EncodeToString(nip44ConversationKeyV2(priv, pub))
		if got != c.ConversationKey {
			t.Errorf("case %d: conversation key = %s, want %s", i, got, c.ConversationKey)
		}
	}
}

func TestNIP44V2GetMessageKeys(t *testing.T) {
	v := loadNIP44Vectors(t)
	convKey := mustHex(t, v.V2.Valid.GetMessageKeys.ConversationKey)
	for i, k := range v.V2.Valid.GetMessageKeys.Keys {
		ck, cn, hk, err := nip44MessageKeysV2(convKey, mustHex(t, k.Nonce))
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if got := hex.EncodeToString(ck); got != k.ChachaKey {
			t.Errorf("case %d: chacha_key = %s, want %s", i, got, k.ChachaKey)
		}
		if got := hex.EncodeToString(cn); got != k.ChachaNonce {
			t.Errorf("case %d: chacha_nonce = %s, want %s", i, got, k.ChachaNonce)
		}
		if got := hex.EncodeToString(hk); got != k.HmacKey {
			t.Errorf("case %d: hmac_key = %s, want %s", i, got, k.HmacKey)
		}
	}
}

func TestNIP44V2CalcPaddedLen(t *testing.T) {
	v := loadNIP44Vectors(t)
	for _, pair := range v.V2.Valid.CalcPaddedLen {
		if got := nip44PadLen(pair[0]); got != pair[1] {
			t.Errorf("padLen(%d) = %d, want %d", pair[0], got, pair[1])
		}
	}
}

func TestNIP44V2EncryptDecrypt(t *testing.T) {
	v := loadNIP44Vectors(t)
	for i, c := range v.V2.Valid.EncryptDecrypt {
		convKey := mustHex(t, c.ConversationKey)

		// The ECDH path must reproduce the stated conversation key.
		priv, err := nip44ParsePriv(mustHex(t, c.Sec1))
		if err != nil {
			t.Fatalf("case %d: parse sec1: %v", i, err)
		}
		pub2 := nip44PubFromPriv(t, c.Sec2)
		if got := hex.EncodeToString(nip44ConversationKeyV2(priv, pub2)); got != c.ConversationKey {
			t.Errorf("case %d: derived conversation key = %s, want %s", i, got, c.ConversationKey)
		}

		gotPayload, err := nip44EncryptV2(c.Plaintext, convKey, mustHex(t, c.Nonce))
		if err != nil {
			t.Fatalf("case %d: encrypt: %v", i, err)
		}
		if gotPayload != c.Payload {
			t.Errorf("case %d: payload = %s, want %s", i, gotPayload, c.Payload)
		}
		gotPlain, err := nip44DecryptV2(c.Payload, convKey)
		if err != nil {
			t.Fatalf("case %d: decrypt: %v", i, err)
		}
		if gotPlain != c.Plaintext {
			t.Errorf("case %d: plaintext = %q, want %q", i, gotPlain, c.Plaintext)
		}
	}
}

func TestNIP44V2LongMessages(t *testing.T) {
	v := loadNIP44Vectors(t)
	for i, c := range v.V2.Valid.EncryptDecryptLongMsg {
		convKey := mustHex(t, c.ConversationKey)
		plaintext := strings.Repeat(c.Pattern, c.Repeat)
		if got := sha256Hex(plaintext); got != c.PlaintextSha256 {
			t.Fatalf("case %d: plaintext sha256 = %s, want %s", i, got, c.PlaintextSha256)
		}
		payload, err := nip44EncryptV2(plaintext, convKey, mustHex(t, c.Nonce))
		if err != nil {
			t.Fatalf("case %d: encrypt: %v", i, err)
		}
		if got := sha256Hex(payload); got != c.PayloadSha256 {
			t.Errorf("case %d: payload sha256 = %s, want %s", i, got, c.PayloadSha256)
		}
		plain, err := nip44DecryptV2(payload, convKey)
		if err != nil {
			t.Fatalf("case %d: decrypt: %v", i, err)
		}
		if plain != plaintext {
			t.Errorf("case %d: round-trip mismatch", i)
		}
	}
}

func TestNIP44V2InvalidConversationKeys(t *testing.T) {
	v := loadNIP44Vectors(t)
	for i, c := range v.V2.Invalid.GetConversationKey {
		sec, errSec := hex.DecodeString(c.Sec1)
		pub, errPub := hex.DecodeString(c.Pub2)
		if errSec != nil || errPub != nil {
			continue // malformed hex is itself a rejection
		}
		_, e1 := nip44ParsePriv(sec)
		_, e2 := nip44ParseXOnlyPub(pub)
		if e1 == nil && e2 == nil {
			t.Errorf("case %d (%s): expected key parse to fail, both succeeded", i, c.Note)
		}
	}
}

func TestNIP44V2InvalidDecrypt(t *testing.T) {
	v := loadNIP44Vectors(t)
	for i, c := range v.V2.Invalid.Decrypt {
		convKey, err := hex.DecodeString(c.ConversationKey)
		if err != nil {
			continue
		}
		if _, err := nip44DecryptV2(c.Payload, convKey); err == nil {
			t.Errorf("case %d (%s): expected decrypt to fail, it succeeded", i, c.Note)
		}
	}
}

func nip44PubFromPriv(t *testing.T, secHex string) *btcec.PublicKey {
	t.Helper()
	priv, err := nip44ParsePriv(mustHex(t, secHex))
	if err != nil {
		t.Fatalf("parse priv %q: %v", secHex, err)
	}
	return priv.PubKey()
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
