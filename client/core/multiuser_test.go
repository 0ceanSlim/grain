package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"golang.org/x/net/websocket"
)

// fakeAuthRelay is an in-process NIP-42 relay. Every connection gets its own
// challenge on connect and is authenticated (as one pubkey) by a valid AUTH.
// Each REQ is answered with one event, signed by the relay's key, whose
// content names the pubkey the connection is authenticated as ("anon" if
// none), so a test can see exactly which identity a request was served as.
type fakeAuthRelay struct {
	t      *testing.T
	srv    *httptest.Server
	url    string
	signer *EventSigner

	mu        sync.Mutex
	published []publishedAs // EVENTs received, with the connection's auth
	conns     []*websocket.Conn
}

type publishedAs struct {
	eventID string
	as      string
}

func newFakeAuthRelay(t *testing.T) *fakeAuthRelay {
	t.Helper()
	signer, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeAuthRelay{t: t, signer: signer}
	r.srv = httptest.NewServer(websocket.Handler(r.serve))
	r.url = "ws://" + strings.TrimPrefix(r.srv.URL, "http://")
	t.Cleanup(r.srv.Close)
	return r
}

// whoAmIFilter is the filter the relay answers with its "served as" event.
func (r *fakeAuthRelay) whoAmIFilter() nostr.Filter {
	return nostr.Filter{Kinds: []int{1}, Authors: []string{r.signer.publicKey}}
}

// dropAll closes every connection from the relay side.
func (r *fakeAuthRelay) dropAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.conns {
		c.Close()
	}
	r.conns = nil
}

func (r *fakeAuthRelay) publishedAs(eventID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.published {
		if p.eventID == eventID {
			return p.as, true
		}
	}
	return "", false
}

func (r *fakeAuthRelay) serve(ws *websocket.Conn) {
	r.mu.Lock()
	r.conns = append(r.conns, ws)
	r.mu.Unlock()

	var writeMu sync.Mutex
	send := func(v ...interface{}) {
		b, _ := json.Marshal(v)
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = websocket.Message.Send(ws, string(b))
	}

	challenge := fmt.Sprintf("chal-%d", time.Now().UnixNano())
	authed := ""
	send("AUTH", challenge)

	for {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			return
		}
		var frame []json.RawMessage
		if json.Unmarshal([]byte(msg), &frame) != nil || len(frame) < 2 {
			continue
		}
		var typ string
		_ = json.Unmarshal(frame[0], &typ)

		switch typ {
		case "AUTH":
			var evt nostr.Event
			_ = json.Unmarshal(frame[1], &evt)
			ok := evt.Kind == 22242 && VerifyEvent(&evt) == nil
			gotChallenge := ""
			for _, tag := range evt.Tags {
				if len(tag) >= 2 && tag[0] == "challenge" {
					gotChallenge = tag[1]
				}
			}
			ok = ok && gotChallenge == challenge
			if ok {
				authed = evt.PubKey
				send("OK", evt.ID, true, "")
			} else {
				send("OK", evt.ID, false, "auth-required: bad AUTH")
			}

		case "REQ":
			var subID string
			_ = json.Unmarshal(frame[1], &subID)
			as := authed
			if as == "" {
				as = "anon"
			}
			evt := nostr.Event{Kind: 1, CreatedAt: time.Now().Unix(), Tags: [][]string{}, Content: as}
			if err := r.signer.SignEvent(&evt); err != nil {
				r.t.Error(err)
				return
			}
			send("EVENT", subID, evt)
			send("EOSE", subID)

		case "EVENT":
			var evt nostr.Event
			_ = json.Unmarshal(frame[1], &evt)
			as := authed
			if as == "" {
				as = "anon"
			}
			r.mu.Lock()
			r.published = append(r.published, publishedAs{evt.ID, as})
			r.mu.Unlock()
			send("OK", evt.ID, true, "")
		}
	}
}

func newSignedUser(t *testing.T, c *Client) *UserContext {
	t.Helper()
	signer, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	return c.NewUserContext(signer.PublicKey(), WithSigner(signer))
}

// servedAs runs the relay's who-am-I query through query and returns the
// identity the relay says it served it as.
func servedAs(t *testing.T, relay *fakeAuthRelay, query func(context.Context, nostr.Filter, []string, ...StreamOption) []*nostr.Event) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	evs := query(ctx, relay.whoAmIFilter(), []string{relay.url}, WithTimeout(5*time.Second))
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	return evs[0].Content
}

// Regression: NIP-42 AUTH used to be sent on the shared pool's socket, so once
// one user authenticated, every other user's requests to that relay were
// served as them (their DMs, gift wraps, and any auth-gated reads).
func TestMultiUser_AuthIsolatedPerUser(t *testing.T) {
	relay := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	defer c.Close()
	alice := newSignedUser(t, c)
	bob := newSignedUser(t, c)
	defer alice.Close()
	defer bob.Close()

	// Everyone starts anonymous; warm the shared connection first so the
	// test also covers "a shared socket already exists when Alice authenticates".
	if got := servedAs(t, relay, c.QueryEvents); got != "anon" {
		t.Fatalf("anonymous query served as %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatalf("alice auth: %v", err)
	}
	if !alice.IsAuthenticated(relay.url) {
		t.Fatal("alice not authenticated after SignAndAuthenticate")
	}
	if bob.IsAuthenticated(relay.url) {
		t.Fatal("bob reports authenticated after alice authenticated")
	}

	if got := servedAs(t, relay, alice.QueryEvents); got != alice.PublicKey() {
		t.Fatalf("alice's query served as %q, want alice", got)
	}
	if got := servedAs(t, relay, bob.QueryEvents); got != "anon" {
		t.Fatalf("bob's query served as %q, want anon (leak of alice's session)", got)
	}
	if got := servedAs(t, relay, c.QueryEvents); got != "anon" {
		t.Fatalf("anonymous query served as %q, want anon (leak of alice's session)", got)
	}

	// Both users authenticated: each is served as themselves.
	if err := bob.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatalf("bob auth: %v", err)
	}
	if got := servedAs(t, relay, bob.QueryEvents); got != bob.PublicKey() {
		t.Fatalf("bob's query served as %q, want bob", got)
	}
	if got := servedAs(t, relay, alice.QueryEvents); got != alice.PublicKey() {
		t.Fatalf("alice's query served as %q after bob authed, want alice", got)
	}
}

func TestMultiUser_PublishGoesOverOwnersConnection(t *testing.T) {
	relay := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	defer c.Close()
	alice := newSignedUser(t, c)
	bob := newSignedUser(t, c)
	defer alice.Close()
	defer bob.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatal(err)
	}

	publish := func(uc *UserContext) string {
		evt := &nostr.Event{Kind: 1, CreatedAt: time.Now().Unix(), Tags: [][]string{}, Content: "hi"}
		if err := uc.Sign(evt); err != nil {
			t.Fatal(err)
		}
		res, err := uc.PublishEvent(ctx, evt, []string{relay.url})
		if err != nil || len(res) != 1 || !res[0].Accepted {
			t.Fatalf("publish: %v %+v", err, res)
		}
		as, ok := relay.publishedAs(evt.ID)
		if !ok {
			t.Fatal("relay never saw the event")
		}
		return as
	}
	if as := publish(alice); as != alice.PublicKey() {
		t.Fatalf("alice's event arrived as %q, want alice", as)
	}
	if as := publish(bob); as != "anon" {
		t.Fatalf("bob's event arrived as %q, want anon", as)
	}

	// The streaming publish path routes the same way.
	evt := &nostr.Event{Kind: 1, CreatedAt: time.Now().Unix(), Tags: [][]string{}, Content: "stream"}
	if err := bob.Sign(evt); err != nil {
		t.Fatal(err)
	}
	for range bob.PublishEventStream(ctx, evt, []string{relay.url}) {
	}
	if as, _ := relay.publishedAs(evt.ID); as != "anon" {
		t.Fatalf("bob's streamed event arrived as %q, want anon", as)
	}
}

func TestMultiUser_SharedPoolNeverAuthenticates(t *testing.T) {
	c := NewClient(DefaultConfig())
	defer c.Close()
	if err := c.SendAuth("wss://relay.example", &nostr.Event{Kind: 22242}); err != ErrSharedAuth {
		t.Fatalf("Client.SendAuth = %v, want ErrSharedAuth", err)
	}
}

func TestMultiUser_AuthenticateRejectsForeignOrStaleEvents(t *testing.T) {
	relay := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	defer c.Close()
	alice := newSignedUser(t, c)
	bob := newSignedUser(t, c)
	defer alice.Close()
	defer bob.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	challenge, err := alice.AuthChallenge(ctx, relay.url)
	if err != nil {
		t.Fatal(err)
	}

	sign := func(uc *UserContext, tags [][]string) *nostr.Event {
		evt := &nostr.Event{Kind: 22242, CreatedAt: time.Now().Unix(), Tags: tags}
		if err := uc.Sign(evt); err != nil {
			t.Fatal(err)
		}
		return evt
	}
	good := [][]string{{"relay", relay.url}, {"challenge", challenge}}

	cases := map[string]*nostr.Event{
		"signed by another user": sign(bob, good),
		"wrong challenge":        sign(alice, [][]string{{"relay", relay.url}, {"challenge", "nope"}}),
		"wrong relay":            sign(alice, [][]string{{"relay", "wss://other.example"}, {"challenge", challenge}}),
	}
	tampered := sign(alice, good)
	tampered.Content = "x"
	cases["bad signature"] = tampered

	for name, evt := range cases {
		if err := alice.Authenticate(ctx, relay.url, evt); err == nil {
			t.Errorf("%s: Authenticate accepted it", name)
		}
	}
	if alice.IsAuthenticated(relay.url) {
		t.Fatal("alice authenticated by a rejected event")
	}
	if err := alice.Authenticate(ctx, relay.url, sign(alice, good)); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
}

func TestMultiUser_DeauthenticateAndReconnect(t *testing.T) {
	relay := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	defer c.Close()
	alice := newSignedUser(t, c)
	defer alice.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatal(err)
	}
	alice.Deauthenticate(relay.url)
	if alice.IsAuthenticated(relay.url) {
		t.Fatal("still authenticated after Deauthenticate")
	}
	if got := servedAs(t, relay, alice.QueryEvents); got != "anon" {
		t.Fatalf("after Deauthenticate served as %q, want anon", got)
	}

	// A dropped connection takes its authentication with it: the redialed
	// socket is a new, unauthenticated one.
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatal(err)
	}
	relay.dropAll()
	deadline := time.Now().Add(5 * time.Second)
	for alice.IsAuthenticated(relay.url) {
		if time.Now().After(deadline) {
			t.Fatal("still authenticated after the relay dropped the connection")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := servedAs(t, relay, alice.QueryEvents); got != "anon" {
		t.Fatalf("after reconnect served as %q, want anon", got)
	}

	// Close ends the session for good.
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatal(err)
	}
	alice.Close()
	if alice.IsAuthenticated(relay.url) {
		t.Fatal("authenticated after Close")
	}
	if err := alice.SignAndAuthenticate(ctx, relay.url); err == nil {
		t.Fatal("authenticated again after Close")
	}
}

func TestMultiUser_AuthStatesPerUser(t *testing.T) {
	relay := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	defer c.Close()
	alice := newSignedUser(t, c)
	bob := newSignedUser(t, c)
	defer alice.Close()
	defer bob.Close()

	// The shared pool sees the relay's challenge: both users are offered it.
	_ = servedAs(t, relay, c.QueryEvents)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatal(err)
	}

	authed := func(uc *UserContext) (bool, bool) {
		for _, st := range uc.AuthStates() {
			if st.Relay == relay.url {
				return true, st.Authed
			}
		}
		return false, false
	}
	if listed, ok := authed(alice); !listed || !ok {
		t.Fatalf("alice: listed=%v authed=%v, want listed and authed", listed, ok)
	}
	if listed, ok := authed(bob); !listed || ok {
		t.Fatalf("bob: listed=%v authed=%v, want listed, not authed", listed, ok)
	}
}

// Regression: relay holds were one Client-wide slot, so each login released
// the previous user's relays.
func TestMultiUser_HoldRelaysIndependent(t *testing.T) {
	r1 := newFakeAuthRelay(t)
	r2 := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	defer c.Close()
	alice := c.NewUserContext("alice")
	bob := c.NewUserContext("bob")

	leases := func(url string) int {
		conn, err := c.relayPool.GetConnection(url)
		if err != nil {
			return 0
		}
		conn.mu.RLock()
		defer conn.mu.RUnlock()
		return conn.leases
	}

	if err := alice.HoldRelays([]string{r1.url}); err != nil {
		t.Fatal(err)
	}
	if err := bob.HoldRelays([]string{r2.url}); err != nil {
		t.Fatal(err)
	}
	if leases(r1.url) != 1 || leases(r2.url) != 1 {
		t.Fatalf("leases r1=%d r2=%d, want 1 each: bob's hold disturbed alice's", leases(r1.url), leases(r2.url))
	}

	// Both hold r1; one leaving must not release the other's.
	if err := bob.HoldRelays([]string{r1.url, r2.url}); err != nil {
		t.Fatal(err)
	}
	if leases(r1.url) != 2 {
		t.Fatalf("r1 leases = %d, want 2", leases(r1.url))
	}
	bob.Close()
	if leases(r1.url) != 1 || leases(r2.url) != 0 {
		t.Fatalf("after bob closed: r1=%d r2=%d, want 1 and 0", leases(r1.url), leases(r2.url))
	}
	if got := alice.HeldRelays(); len(got) != 1 || got[0] != r1.url {
		t.Fatalf("alice holds %v, want [%s]", got, r1.url)
	}
	if err := bob.HoldRelays([]string{r2.url}); err == nil {
		t.Fatal("HoldRelays succeeded after Close")
	}
	alice.ReleaseRelays()
	if leases(r1.url) != 0 {
		t.Fatalf("r1 leases after ReleaseRelays = %d, want 0", leases(r1.url))
	}
}

// Regression: fixed-relay mode and app relays were Client-wide, so one user
// enabling them rerouted every user's reads and writes.
func TestMultiUser_RoutingPrefsPerUser(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IndexRelays = []string{"wss://index.example"}
	c := NewClient(cfg)
	defer c.Close()
	alice := c.NewUserContext("alice")
	bob := c.NewUserContext("bob")

	alice.PinFixedRelays([]string{"wss://alice-read.example"}, []string{"wss://alice-write.example"})
	alice.SetAppRelays(RoleBroadcast, []string{"wss://alice-blast.example"})

	if !alice.FixedRelaysEnabled() || bob.FixedRelaysEnabled() || c.FixedRelaysEnabled() {
		t.Fatalf("fixed: alice=%v bob=%v client=%v, want only alice",
			alice.FixedRelaysEnabled(), bob.FixedRelaysEnabled(), c.FixedRelaysEnabled())
	}
	if got := alice.RouteFetch("someone"); len(got) != 1 || got[0] != "wss://alice-read.example" {
		t.Fatalf("alice RouteFetch = %v", got)
	}
	if got := bob.RouteFetch("someone"); len(got) != 1 || got[0] != "wss://index.example" {
		t.Fatalf("bob RouteFetch = %v, want index fallback", got)
	}
	evt := &nostr.Event{PubKey: "bob", Kind: 1}
	for _, u := range bob.RoutePublish(evt) {
		if strings.Contains(u, "alice") {
			t.Fatalf("bob's publish routed to alice's relay %s", u)
		}
	}
	if got := bob.AppRelays(RoleBroadcast); len(got) != 0 {
		t.Fatalf("bob broadcast relays = %v, want none", got)
	}

	// App-wide defaults still apply to users without an override, and a user
	// can opt back out of an app-wide fixed mode.
	c.SetFixedRelays([]string{"wss://app-read.example"}, nil)
	if got := bob.RouteFetch("someone"); len(got) != 1 || got[0] != "wss://app-read.example" {
		t.Fatalf("bob RouteFetch under app-wide fixed = %v", got)
	}
	bob.ClearFixedRelays()
	if bob.FixedRelaysEnabled() {
		t.Fatal("bob still fixed after ClearFixedRelays")
	}
	if got := alice.RouteFetch("someone"); got[0] != "wss://alice-read.example" {
		t.Fatalf("alice's own fixed set lost to the app default: %v", got)
	}
}

// Client.Close ends every user's private connections, even for contexts the
// app never closed.
func TestMultiUser_ClientCloseEndsUserSessions(t *testing.T) {
	relay := newFakeAuthRelay(t)
	c := NewClient(DefaultConfig())
	alice := newSignedUser(t, c)
	_ = c.NewUserContext("read-only") // no private pool: never tracked

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := alice.SignAndAuthenticate(ctx, relay.url); err != nil {
		t.Fatal(err)
	}
	c.lanesMu.Lock()
	tracked := len(c.lanes)
	c.lanesMu.Unlock()
	if tracked != 1 {
		t.Fatalf("tracking %d user pools, want 1", tracked)
	}
	c.Close()
	if alice.IsAuthenticated(relay.url) {
		t.Fatal("alice still authenticated after Client.Close")
	}
	c.lanesMu.Lock()
	tracked = len(c.lanes)
	c.lanesMu.Unlock()
	if tracked != 0 {
		t.Fatalf("still tracking %d user pools after Close", tracked)
	}
}
