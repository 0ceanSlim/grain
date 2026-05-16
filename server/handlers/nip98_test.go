package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// testKeypair is a minimal in-package mirror of tests.TestKeypair. We
// keep it local so this file can live in server/handlers without a
// test-only dependency on the integration tests package.
type testKeypair struct {
	priv   *btcec.PrivateKey
	pubHex string
}

func newTestKeypair(t *testing.T) *testKeypair {
	t.Helper()
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return &testKeypair{
		priv:   priv,
		pubHex: hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey())),
	}
}

// signEventAt builds and signs an event with an explicit created_at
// so tests can exercise the time-window boundaries directly.
func (kp *testKeypair) signEventAt(t *testing.T, kind int, content string, tags [][]string, createdAt int64) nostr.Event {
	t.Helper()
	if tags == nil {
		tags = [][]string{}
	}
	evt := nostr.Event{
		PubKey:    kp.pubHex,
		CreatedAt: createdAt,
		Kind:      kind,
		Tags:      tags,
		Content:   content,
	}
	serialized, _ := json.Marshal([]interface{}{
		0, evt.PubKey, evt.CreatedAt, evt.Kind, evt.Tags, evt.Content,
	})
	h := sha256.Sum256(serialized)
	evt.ID = hex.EncodeToString(h[:])
	sig, err := schnorr.Sign(kp.priv, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	evt.Sig = hex.EncodeToString(sig.Serialize())
	return evt
}

func (kp *testKeypair) signEvent(t *testing.T, kind int, content string, tags [][]string) nostr.Event {
	return kp.signEventAt(t, kind, content, tags, time.Now().Unix())
}

// withAuthConfig installs an AuthConfig for the duration of a test
// and restores whatever was there before. The whole-config swap is
// crude but mirrors how production code reads cfg, so the call site
// in VerifyNIP98Event isn't exercising a test-only path.
func withAuthConfig(t *testing.T, ac cfgType.AuthConfig) {
	t.Helper()
	prev := config.GetConfig()
	swap := &cfgType.ServerConfig{}
	if prev != nil {
		swap = prev
	}
	saved := swap.Auth
	swap.Auth = ac
	config.SetConfigForTesting(swap)
	t.Cleanup(func() {
		swap.Auth = saved
		config.SetConfigForTesting(prev)
	})
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func nip98Tags(method, u, payload string) [][]string {
	tags := [][]string{{"method", method}, {"u", u}}
	if payload != "" {
		tags = append(tags, []string{"payload", payload})
	}
	return tags
}

func TestVerifyNIP98Event_HappyGET(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/api/v1/relay/whitelist/pubkeys"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", url, ""))
	if err := VerifyNIP98Event(evt, "GET", url, ""); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestVerifyNIP98Event_HappyPOSTWithBody(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/api/v1/relay/whitelist/pubkeys"
	body := []byte(`{"pubkeys":["abc"]}`)
	hash := sha256Hex(body)
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("POST", url, hash))
	if err := VerifyNIP98Event(evt, "POST", url, hash); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestVerifyNIP98Event_WrongKind(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, 22242, "", nip98Tags("GET", url, ""))
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "kind must be 27235") {
		t.Fatalf("expected kind error, got %v", err)
	}
}

func TestVerifyNIP98Event_TooOld(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEventAt(t, NIP98AuthKind, "", nip98Tags("GET", url, ""), time.Now().Add(-2*time.Minute).Unix())
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "too old") {
		t.Fatalf("expected too-old error, got %v", err)
	}
}

func TestVerifyNIP98Event_FutureCreatedAt(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEventAt(t, NIP98AuthKind, "", nip98Tags("GET", url, ""), time.Now().Add(2*time.Minute).Unix())
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("expected future error, got %v", err)
	}
}

func TestVerifyNIP98Event_MissingUTag(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, NIP98AuthKind, "", [][]string{{"method", "GET"}})
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "u tag missing") {
		t.Fatalf("expected u-missing error, got %v", err)
	}
}

func TestVerifyNIP98Event_MissingMethodTag(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, NIP98AuthKind, "", [][]string{{"u", url}})
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "method tag missing") {
		t.Fatalf("expected method-missing error, got %v", err)
	}
}

func TestVerifyNIP98Event_MethodMismatch(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", url, ""))
	err := VerifyNIP98Event(evt, "POST", url, "")
	if err == nil || !strings.Contains(err.Error(), "method tag does not match") {
		t.Fatalf("expected method-mismatch error, got %v", err)
	}
}

func TestVerifyNIP98Event_URLMismatchStrict(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	signedURL := "https://relay.example/foo"
	requestURL := "https://relay.example/bar"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", signedURL, ""))
	err := VerifyNIP98Event(evt, "GET", requestURL, "")
	if err == nil || !strings.Contains(err.Error(), "u tag does not match") {
		t.Fatalf("expected u-mismatch error, got %v", err)
	}
}

func TestVerifyNIP98Event_StrictQueryMismatch(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	signedURL := "https://relay.example/foo?a=1"
	requestURL := "https://relay.example/foo?a=2"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", signedURL, ""))
	err := VerifyNIP98Event(evt, "GET", requestURL, "")
	if err == nil || !strings.Contains(err.Error(), "u tag does not match") {
		t.Fatalf("expected query-mismatch error, got %v", err)
	}
}

func TestVerifyNIP98Event_HostModeIgnoresPath(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "host"})
	kp := newTestKeypair(t)
	signedURL := "https://relay.example/foo"
	requestURL := "https://relay.example/bar?x=1"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", signedURL, ""))
	if err := VerifyNIP98Event(evt, "GET", requestURL, ""); err != nil {
		t.Fatalf("expected host-mode pass, got %v", err)
	}
}

func TestVerifyNIP98Event_PayloadMissingButBodyPresent(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	body := []byte("hello")
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("POST", url, ""))
	err := VerifyNIP98Event(evt, "POST", url, sha256Hex(body))
	if err == nil || !strings.Contains(err.Error(), "payload tag missing") {
		t.Fatalf("expected payload-missing error, got %v", err)
	}
}

func TestVerifyNIP98Event_PayloadMismatch(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("POST", url, sha256Hex([]byte("signed"))))
	err := VerifyNIP98Event(evt, "POST", url, sha256Hex([]byte("actual")))
	if err == nil || !strings.Contains(err.Error(), "payload tag does not match") {
		t.Fatalf("expected payload-mismatch error, got %v", err)
	}
}

func TestVerifyNIP98Event_PayloadPresentButNoBody(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", url, sha256Hex([]byte("ghost"))))
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "payload tag present but request has no body") {
		t.Fatalf("expected payload-without-body error, got %v", err)
	}
}

func TestVerifyNIP98Event_BadSignature(t *testing.T) {
	withAuthConfig(t, cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"})
	kp := newTestKeypair(t)
	url := "https://relay.example/foo"
	evt := kp.signEvent(t, NIP98AuthKind, "", nip98Tags("GET", url, ""))
	// Flip a byte in the signature so verification fails. We replace
	// the leading hex digit with a guaranteed-different value so the
	// signature stays a valid hex string of the right length.
	if evt.Sig[0] == 'a' {
		evt.Sig = "b" + evt.Sig[1:]
	} else {
		evt.Sig = "a" + evt.Sig[1:]
	}
	err := VerifyNIP98Event(evt, "GET", url, "")
	if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("expected sig error, got %v", err)
	}
}
