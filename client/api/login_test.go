package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
)

const loginURL = "http://relay.example/api/v1/auth/login"

// nip98 signs a login Authorization header over body.
func nip98(t *testing.T, s *core.EventSigner, body string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(body))
	evt := &nostr.Event{
		Kind:      27235,
		CreatedAt: time.Now().Unix(),
		Tags: [][]string{
			{"u", loginURL},
			{"method", "POST"},
			{"payload", hex.EncodeToString(sum[:])},
		},
	}
	if err := s.SignEvent(evt); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(evt)
	return "Nostr " + base64.StdEncoding.EncodeToString(raw)
}

// A write login must be signed by the pubkey it names, once; an unsigned
// login only gets a read-only, unverified session.
func TestLogin_RequiresSignatureForWrite(t *testing.T) {
	cfg := &cfgType.ServerConfig{}
	cfg.Client.IndexRelays = []string{"ws://127.0.0.1:1"}
	config.SetConfigForTesting(cfg)
	t.Cleanup(func() { config.SetConfigForTesting(nil) })
	if err := connection.InitializeCoreClient(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.CloseCoreClient() })
	session.SessionMgr = session.NewSessionManager()
	t.Cleanup(func() { session.SessionMgr = nil })

	alice, _ := core.NewEventSignerFromRandom()
	mallory, _ := core.NewEventSignerFromRandom()

	post := func(body, auth string) (int, *session.Response) {
		req := httptest.NewRequest("POST", loginURL, strings.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		LoginHandler(rec, req)
		var out session.Response
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, &out
	}
	body := func(mode string) string {
		return `{"public_key":"` + alice.PublicKey() + `","requested_mode":"` + mode + `","signing_method":"browser_extension"}`
	}

	if code, _ := post(body("write"), ""); code != http.StatusUnauthorized {
		t.Errorf("unsigned write login: HTTP %d, want 401", code)
	}
	if code, _ := post(body("write"), nip98(t, mallory, body("write"))); code != http.StatusUnauthorized {
		t.Errorf("write login signed by another key: HTTP %d, want 401", code)
	}
	if code, _ := post(body("write"), nip98(t, alice, body("read_only"))); code != http.StatusUnauthorized {
		t.Errorf("signature over a different body: HTTP %d, want 401", code)
	}

	signed := nip98(t, alice, body("write"))
	code, out := post(body("write"), signed)
	if code != http.StatusOK || out.Session == nil || !out.Session.Verified {
		t.Fatalf("signed write login: HTTP %d %+v", code, out)
	}
	if code, _ := post(body("write"), signed); code != http.StatusUnauthorized {
		t.Errorf("replayed login signature: HTTP %d, want 401", code)
	}

	code, out = post(body("read_only"), "")
	if code != http.StatusOK || out.Session == nil || out.Session.Verified || out.Session.Mode != session.ReadOnlyMode {
		t.Fatalf("read-only login: HTTP %d %+v", code, out)
	}

	bad := `{"public_key":"` + strings.Repeat("z", 64) + `","requested_mode":"read_only"}`
	if code, _ := post(bad, ""); code != http.StatusBadRequest {
		t.Errorf("non-hex pubkey: HTTP %d, want 400", code)
	}
}
