package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"golang.org/x/net/websocket"
)

// testUser is a logged-in browser: a keypair and its session cookie.
type testUser struct {
	signer *core.EventSigner
	cookie *http.Cookie
}

func (u testUser) pubkey() string { return u.signer.PublicKey() }

// setupPerUser starts a core client (whose only index relay is a closed port,
// so nothing dials out) and a session manager, and logs in two users.
func setupPerUser(t *testing.T) (alice, bob testUser) {
	t.Helper()
	cfg := &cfgType.ServerConfig{}
	cfg.Client.IndexRelays = []string{"ws://127.0.0.1:1"}
	if err := connection.InitializeCoreClient(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.CloseCoreClient() })
	session.SessionMgr = session.NewSessionManager()
	t.Cleanup(func() { session.SessionMgr = nil })

	login := func() testUser {
		signer, err := core.NewEventSignerFromRandom()
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		_, err = session.SessionMgr.CreateSession(rec, httptest.NewRequest("POST", "/", nil),
			session.SessionInitRequest{PublicKey: signer.PublicKey(), RequestedMode: session.WriteMode}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return testUser{signer: signer, cookie: rec.Result().Cookies()[0]}
	}
	return login(), login()
}

// call runs handler as user (nil = anonymous) and decodes the JSON response.
func call(t *testing.T, handler http.HandlerFunc, user *testUser, method string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, "/", &buf)
	if user != nil {
		req.AddCookie(user.cookie)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// Regression: the fixed-relay and app-relay endpoints wrote Client-wide state,
// so any logged-in user could reroute every other user's reads and writes.
func TestRoutingPrefsEndpoints_PerUser(t *testing.T) {
	alice, bob := setupPerUser(t)

	code, out := call(t, FixedRelaysHandler, &alice, "POST",
		FixedRelaysRequest{Enabled: true, Read: []string{"wss://a.example"}, Write: []string{"wss://a.example"}})
	if code != 200 || out["enabled"] != true {
		t.Fatalf("alice fixed-relays: %d %v", code, out)
	}
	if !connection.UserFor(alice.pubkey()).FixedRelaysEnabled() {
		t.Fatal("alice's fixed mode not set")
	}
	if connection.UserFor(bob.pubkey()).FixedRelaysEnabled() || connection.GetCoreClient().FixedRelaysEnabled() {
		t.Fatal("alice's fixed mode leaked to bob or the whole client")
	}

	code, _ = call(t, AppRelaysHandler, &alice, "POST", AppRelaysPayload{Broadcast: []string{"wss://blast.example"}})
	if code != 200 {
		t.Fatalf("alice app-relays: %d", code)
	}
	_, got := call(t, AppRelaysHandler, &bob, "GET", nil)
	if b, _ := got["broadcast"].([]interface{}); len(b) != 0 {
		t.Fatalf("bob sees alice's broadcast relays: %v", got["broadcast"])
	}
	if b := connection.GetCoreClient().AppRelays(core.RoleBroadcast); len(b) != 0 {
		t.Fatalf("alice's broadcast relays became app-wide: %v", b)
	}

	if code, _ := call(t, FixedRelaysHandler, nil, "POST", FixedRelaysRequest{Enabled: true}); code != http.StatusUnauthorized {
		t.Fatalf("anonymous fixed-relays: %d, want 401", code)
	}
}

// fakeAuthRelay serves a NIP-42 challenge on connect and OKs any validly
// signed AUTH that answers it.
func fakeAuthRelay(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		challenge := "c-" + time.Now().Format("150405.000000000")
		send := func(v ...interface{}) {
			b, _ := json.Marshal(v)
			_ = websocket.Message.Send(ws, string(b))
		}
		send("AUTH", challenge)
		for {
			var msg string
			if websocket.Message.Receive(ws, &msg) != nil {
				return
			}
			var frame []json.RawMessage
			if json.Unmarshal([]byte(msg), &frame) != nil || len(frame) < 2 {
				continue
			}
			var typ string
			_ = json.Unmarshal(frame[0], &typ)
			if typ != "AUTH" {
				continue
			}
			var evt nostr.Event
			_ = json.Unmarshal(frame[1], &evt)
			send("OK", evt.ID, core.VerifyEvent(&evt) == nil, "")
		}
	}))
	t.Cleanup(srv.Close)
	return "ws://" + strings.TrimPrefix(srv.URL, "http://")
}

// Regression: NIP-42 AUTH went out on the shared pool's socket, so one user
// authenticating made the relay serve every user as them. Now each user gets
// a challenge on their own connection and only their state changes.
func TestNIP42Endpoints_PerUser(t *testing.T) {
	alice, bob := setupPerUser(t)
	relay := fakeAuthRelay(t)

	code, out := call(t, AuthChallengeHandler, &alice, "POST", map[string]string{"relay": relay})
	challenge, _ := out["challenge"].(string)
	if code != 200 || challenge == "" {
		t.Fatalf("alice challenge: %d %v", code, out)
	}

	sign := func(u testUser) *nostr.Event {
		evt := &nostr.Event{Kind: 22242, CreatedAt: time.Now().Unix(),
			Tags: [][]string{{"relay", relay}, {"challenge", challenge}}}
		if err := u.signer.SignEvent(evt); err != nil {
			t.Fatal(err)
		}
		return evt
	}

	// Bob can't answer with an event signed by Alice, nor Alice with Bob's.
	_, out = call(t, SubmitAuthHandler, &bob, "POST", map[string]interface{}{"relay": relay, "event": sign(alice)})
	if out["success"] == true {
		t.Fatal("bob authenticated with alice's AUTH event")
	}
	_, out = call(t, SubmitAuthHandler, &alice, "POST", map[string]interface{}{"relay": relay, "event": sign(bob)})
	if out["success"] == true {
		t.Fatal("alice authenticated with bob's AUTH event")
	}

	_, out = call(t, SubmitAuthHandler, &alice, "POST", map[string]interface{}{"relay": relay, "event": sign(alice)})
	if out["success"] != true {
		t.Fatalf("alice AUTH failed: %v", out)
	}

	authedFor := func(u testUser) bool {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(u.cookie)
		rec := httptest.NewRecorder()
		AuthRequestsHandler(rec, req)
		var states []core.AuthState
		_ = json.Unmarshal(rec.Body.Bytes(), &states)
		for _, st := range states {
			if st.Relay == relay {
				return st.Authed
			}
		}
		return false
	}
	if !authedFor(alice) {
		t.Fatal("alice not listed as authed")
	}
	if authedFor(bob) {
		t.Fatal("bob listed as authed after alice authenticated")
	}

	// Revoking is Alice's alone too.
	call(t, RemoveAuthHandler, &alice, "POST", map[string]string{"relay": relay})
	if authedFor(alice) {
		t.Fatal("alice still authed after remove")
	}
}

// Regression: logging out never released anything, and logging in released
// the previous user's relays. A user's held relays and AUTH sessions now live
// until their own last session ends.
func TestLogoutReleasesOnlyThatUser(t *testing.T) {
	alice, bob := setupPerUser(t)
	ucA := connection.UserFor(alice.pubkey())
	ucB := connection.UserFor(bob.pubkey())

	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(alice.cookie)
	session.SessionMgr.ClearSession(httptest.NewRecorder(), req)

	if err := ucA.HoldRelays([]string{"wss://x.example"}); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("alice's context still open after logout: %v", err)
	}
	if err := ucB.HoldRelays(nil); err != nil {
		t.Fatalf("bob's context closed by alice's logout: %v", err)
	}
	if connection.UserFor(alice.pubkey()) == ucA {
		t.Fatal("alice's released context is still registered")
	}
}
