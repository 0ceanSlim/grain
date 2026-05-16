package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/handlers"
	nostr "github.com/0ceanslim/grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// signEvent here is a self-contained NIP-98 signer so the api test
// file doesn't reach into the test helpers in tests/, which would
// pull integration-test dependencies into the unit build.
type kp struct {
	priv *btcec.PrivateKey
	pub  string
}

func newKP(t *testing.T) *kp {
	t.Helper()
	p, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return &kp{priv: p, pub: hex.EncodeToString(schnorr.SerializePubKey(p.PubKey()))}
}

func (k *kp) sign(t *testing.T, method, url, payload string) nostr.Event {
	t.Helper()
	tags := [][]string{{"method", method}, {"u", url}}
	if payload != "" {
		tags = append(tags, []string{"payload", payload})
	}
	evt := nostr.Event{
		PubKey:    k.pub,
		CreatedAt: time.Now().Unix(),
		Kind:      handlers.NIP98AuthKind,
		Tags:      tags,
		Content:   "",
	}
	ser, _ := json.Marshal([]interface{}{0, evt.PubKey, evt.CreatedAt, evt.Kind, evt.Tags, evt.Content})
	h := sha256.Sum256(ser)
	evt.ID = hex.EncodeToString(h[:])
	sig, err := schnorr.Sign(k.priv, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	evt.Sig = hex.EncodeToString(sig.Serialize())
	return evt
}

func authHeader(t *testing.T, evt nostr.Event) string {
	t.Helper()
	b, _ := json.Marshal(evt)
	return "Nostr " + base64.StdEncoding.EncodeToString(b)
}

func installAuthCfg(t *testing.T) {
	t.Helper()
	cfg := &cfgType.ServerConfig{Auth: cfgType.AuthConfig{RelayURL: "https://relay.example/", RelayURLMatch: "strict"}}
	prev := config.GetConfig()
	config.SetConfigForTesting(cfg)
	t.Cleanup(func() { config.SetConfigForTesting(prev) })
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestExtractNIP98Event_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://relay.example/foo", nil)
	_, err := ExtractNIP98Event(r)
	if !errors.Is(err, ErrMissingAuthHeader) {
		t.Fatalf("expected ErrMissingAuthHeader, got %v", err)
	}
}

func TestExtractNIP98Event_WrongScheme(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://relay.example/foo", nil)
	r.Header.Set("Authorization", "Bearer abc")
	_, err := ExtractNIP98Event(r)
	if err == nil || !strings.Contains(err.Error(), "Nostr scheme") {
		t.Fatalf("expected scheme error, got %v", err)
	}
}

func TestExtractNIP98Event_BadBase64(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://relay.example/foo", nil)
	r.Header.Set("Authorization", "Nostr !!!not-base64!!!")
	_, err := ExtractNIP98Event(r)
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("expected base64 error, got %v", err)
	}
}

func TestExtractNIP98Event_BadJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://relay.example/foo", nil)
	r.Header.Set("Authorization", "Nostr "+base64.StdEncoding.EncodeToString([]byte("not json")))
	_, err := ExtractNIP98Event(r)
	if err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("expected json error, got %v", err)
	}
}

func TestExtractNIP98Event_CaseInsensitiveScheme(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://relay.example/foo", nil)
	r.Header.Set("Authorization", "nostr "+base64.StdEncoding.EncodeToString([]byte(`{"kind":27235}`)))
	if _, err := ExtractNIP98Event(r); err != nil {
		t.Fatalf("expected case-insensitive scheme to parse, got %v", err)
	}
}

func TestVerifyAPIAuth_HappyGET(t *testing.T) {
	installAuthCfg(t)
	k := newKP(t)
	url := "https://relay.example/api/v1/relay/whitelist/pubkeys"
	evt := k.sign(t, "GET", url, "")
	r := httptest.NewRequest(http.MethodGet, url, nil)
	r.Header.Set("Authorization", authHeader(t, evt))
	pub, err := VerifyAPIAuth(r)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pub != k.pub {
		t.Fatalf("pub mismatch: got %q want %q", pub, k.pub)
	}
}

func TestVerifyAPIAuth_HappyPOSTWithBody(t *testing.T) {
	installAuthCfg(t)
	k := newKP(t)
	url := "https://relay.example/api/v1/relay/whitelist/pubkeys"
	body := []byte(`{"pubkeys":["abc"]}`)
	evt := k.sign(t, "POST", url, sha256hex(body))
	r := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	r.Header.Set("Authorization", authHeader(t, evt))
	pub, err := VerifyAPIAuth(r)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pub != k.pub {
		t.Fatalf("pub mismatch: got %q want %q", pub, k.pub)
	}
	// The handler must still be able to read the body — VerifyAPIAuth
	// is contractually required to restore it after hashing.
	got, _ := io.ReadAll(r.Body)
	if !bytes.Equal(got, body) {
		t.Fatalf("body not restored: got %q want %q", got, body)
	}
}

func TestVerifyAPIAuth_BodyTamperedAfterSign(t *testing.T) {
	installAuthCfg(t)
	k := newKP(t)
	url := "https://relay.example/foo"
	signedBody := []byte(`{"pubkeys":["abc"]}`)
	tamperedBody := []byte(`{"pubkeys":["abc","attacker"]}`)
	evt := k.sign(t, "POST", url, sha256hex(signedBody))
	r := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(tamperedBody))
	r.Header.Set("Authorization", authHeader(t, evt))
	if _, err := VerifyAPIAuth(r); err == nil {
		t.Fatalf("expected hash mismatch to be rejected")
	}
}

func TestAbsoluteRequestURL_ProxyHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/relay/whitelist?x=1", nil)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "relay.example.com")
	got := absoluteRequestURL(r)
	want := "https://relay.example.com/api/v1/relay/whitelist?x=1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAbsoluteRequestURL_NoProxy(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/foo", nil)
	r.Host = "relay.example:8080"
	got := absoluteRequestURL(r)
	want := "http://relay.example:8080/foo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAbsoluteRequestURL_ForwardedHostChain(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/foo", nil)
	r.Host = "internal"
	r.Header.Set("X-Forwarded-Proto", "https, http")
	r.Header.Set("X-Forwarded-Host", "relay.example.com, mid-proxy")
	got := absoluteRequestURL(r)
	want := "https://relay.example.com/foo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRequireOwner_NotOwnerReturns403(t *testing.T) {
	installAuthCfg(t)
	k := newKP(t)
	url := "https://relay.example/admin"
	evt := k.sign(t, "GET", url, "")
	r := httptest.NewRequest(http.MethodGet, url, nil)
	r.Header.Set("Authorization", authHeader(t, evt))
	w := httptest.NewRecorder()
	// No owner pubkey loaded — IsRelayOwner returns false for any
	// signer, which is exactly the gate we want.
	if _, ok := RequireOwner(w, r); ok {
		t.Fatalf("expected RequireOwner to deny when signer is not owner")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireOwner_NoAuthHeaderReturns401(t *testing.T) {
	installAuthCfg(t)
	r := httptest.NewRequest(http.MethodGet, "https://relay.example/admin", nil)
	w := httptest.NewRecorder()
	if _, ok := RequireOwner(w, r); ok {
		t.Fatalf("expected RequireOwner to deny with no auth header")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") != "Nostr" {
		t.Fatalf("expected WWW-Authenticate: Nostr, got %q", w.Header().Get("WWW-Authenticate"))
	}
}

func TestHashAndRestoreBody_OversizedRejected(t *testing.T) {
	big := bytes.Repeat([]byte{'a'}, maxAuthBodyBytes+1)
	r := httptest.NewRequest(http.MethodPost, "https://relay.example/foo", bytes.NewReader(big))
	_, err := hashAndRestoreBody(r)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size error, got %v", err)
	}
}
