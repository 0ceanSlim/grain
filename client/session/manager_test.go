package session

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func newTestSession(t *testing.T, sm *SessionManager, r *http.Request) (token string, cookie *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	if _, err := sm.CreateSession(rec, r, SessionInitRequest{PublicKey: "pk", RequestedMode: WriteMode}, []string{"wss://a"}); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	return cookies[0].Value, cookies[0]
}

// Regression: GetUserSession wrote LastActive under a read lock, so concurrent
// requests on one session raced (and so did anything reading the returned
// pointer). Run with -race.
func TestGetUserSession_ConcurrentAccess(t *testing.T) {
	sm := NewSessionManager()
	token, _ := newTestSession(t, sm, httptest.NewRequest("GET", "/", nil))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				s := sm.GetUserSession(token)
				if s == nil {
					t.Error("session vanished")
					return
				}
				_ = s.LastActive.Unix() + int64(len(s.ConnectedRelays))
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			sm.CleanupSessions(time.Hour)
			_ = sm.GetSessionStats()
		}
	}()
	wg.Wait()
}

func TestGetUserSession_ReturnsSnapshot(t *testing.T) {
	sm := NewSessionManager()
	token, _ := newTestSession(t, sm, httptest.NewRequest("GET", "/", nil))

	s := sm.GetUserSession(token)
	s.Mode = ReadOnlyMode // must not write through to the stored session
	if got := sm.GetUserSession(token); got.Mode != WriteMode {
		t.Fatalf("mutating the returned session changed the stored one: mode=%s", got.Mode)
	}
	if got := sm.GetUserSession(token); len(got.ConnectedRelays) != 1 {
		t.Fatalf("ConnectedRelays = %v, want the relays passed to CreateSession", got.ConnectedRelays)
	}
}

// Regression: the session cookie was hardcoded Secure=false, so on an HTTPS
// deployment it could still be sent over plain http.
func TestSessionCookie_SecureOverHTTPS(t *testing.T) {
	plain := httptest.NewRequest("GET", "http://localhost/", nil)

	direct := httptest.NewRequest("GET", "https://relay.example/", nil)
	direct.TLS = &tls.ConnectionState{}

	proxied := httptest.NewRequest("GET", "http://relay.example/", nil)
	proxied.Header.Set("X-Forwarded-Proto", "HTTPS")

	chained := httptest.NewRequest("GET", "http://relay.example/", nil)
	chained.Header.Set("X-Forwarded-Proto", "https, http")

	proxiedHTTP := httptest.NewRequest("GET", "http://relay.example/", nil)
	proxiedHTTP.Header.Set("X-Forwarded-Proto", "http")

	cases := []struct {
		name   string
		r      *http.Request
		secure bool
	}{
		{"plain http (local dev)", plain, false},
		{"direct TLS", direct, true},
		{"behind TLS proxy", proxied, true},
		{"proxy chain, client hop https", chained, true},
		{"behind plain proxy", proxiedHTTP, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sm := NewSessionManager()
			_, c := newTestSession(t, sm, tc.r)
			if c.Secure != tc.secure {
				t.Errorf("create: Secure=%v, want %v", c.Secure, tc.secure)
			}
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				t.Errorf("create: lost HttpOnly/SameSite: %+v", c)
			}

			rec := httptest.NewRecorder()
			tc.r.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
			sm.ClearSession(rec, tc.r)
			cleared := rec.Result().Cookies()
			if len(cleared) != 1 || cleared[0].Secure != tc.secure || cleared[0].MaxAge >= 0 {
				t.Errorf("clear: got %+v, want one expiring cookie with Secure=%v", cleared, tc.secure)
			}
		})
	}
}

// A user's per-user client state is released when their LAST session ends
// (logout or expiry), not when one of several sessions does.
func TestSessionEnd_ReleasesUserOnLastSession(t *testing.T) {
	sm := NewSessionManager()
	var released []string
	sm.onUserGone = func(pk string) { released = append(released, pk) }

	create := func(pk string) *http.Cookie {
		rec := httptest.NewRecorder()
		if _, err := sm.CreateSession(rec, httptest.NewRequest("GET", "/", nil), SessionInitRequest{PublicKey: pk, Verified: true}, nil); err != nil {
			t.Fatal(err)
		}
		return rec.Result().Cookies()[0]
	}
	logout := func(c *http.Cookie) {
		r := httptest.NewRequest("POST", "/", nil)
		r.AddCookie(c)
		sm.ClearSession(httptest.NewRecorder(), r)
	}

	phone, laptop := create("alice"), create("alice")
	create("bob")

	logout(phone)
	if len(released) != 0 {
		t.Fatalf("released %v while alice still has a session", released)
	}
	logout(laptop)
	if len(released) != 1 || released[0] != "alice" {
		t.Fatalf("released %v, want [alice]", released)
	}

	// Expiry releases too.
	sm.CleanupSessions(-time.Second)
	if len(released) != 2 || released[1] != "bob" {
		t.Fatalf("released %v after expiry, want bob too", released)
	}
}

// An unsigned session never shares client state with the pubkey it names;
// signed sessions of one pubkey do.
func TestUserKey_UnverifiedSessionsAreIsolated(t *testing.T) {
	sm := NewSessionManager()
	get := func(verified bool) *UserSession {
		rec := httptest.NewRecorder()
		s, err := sm.CreateSession(rec, httptest.NewRequest("POST", "/", nil),
			SessionInitRequest{PublicKey: "alice", RequestedMode: ReadOnlyMode, Verified: verified}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	signed1, signed2 := get(true), get(true)
	view1, view2 := get(false), get(false)
	if signed1.UserKey() != "alice" || signed2.UserKey() != "alice" {
		t.Fatalf("signed sessions use %q and %q, want alice", signed1.UserKey(), signed2.UserKey())
	}
	if view1.UserKey() == "alice" || view1.UserKey() == view2.UserKey() {
		t.Fatalf("unsigned sessions use %q and %q: shared state", view1.UserKey(), view2.UserKey())
	}
}

// The session table is capped; at the cap the oldest unsigned session goes
// before any signed one, and its client state is released.
func TestCreateSession_CapEvictsUnverifiedFirst(t *testing.T) {
	sm := NewSessionManager()
	sm.maxSessions = 3
	var released []string
	sm.onUserGone = func(k string) { released = append(released, k) }
	create := func(pk string, verified bool) *UserSession {
		s, err := sm.CreateSession(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil),
			SessionInitRequest{PublicKey: pk, Verified: verified}, nil)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // distinct LastActive
		return s
	}
	create("old-signed", true)
	view := create("viewer", false)
	create("signed", true)
	for i := 0; i < 5; i++ {
		create("new", true)
	}
	if n := sm.GetSessionStats()["total_sessions"].(int); n != 3 {
		t.Fatalf("%d sessions, want the cap of 3", n)
	}
	if len(released) == 0 || released[0] != view.UserKey() {
		t.Fatalf("released %v, want the unsigned session %q first", released, view.UserKey())
	}
	if sm.HasUserKey(view.UserKey()) || sm.HasUserKey("old-signed") {
		t.Fatal("the oldest sessions survived the cap")
	}
}
