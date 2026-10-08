package core

import (
	"context"
	"fmt"
	"sort"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// NIP-42 AUTH (https://github.com/nostr-protocol/nips/blob/master/42.md).
//
// A relay authenticates a *connection*: once a socket answers a challenge, the
// relay serves everything sent on that socket as the authenticated pubkey. So
// a socket that has authenticated must only ever carry its owner's traffic.
//
// The Client's shared pool therefore never authenticates. It still records the
// challenges relays send it, which tells the app which relays want AUTH. To
// authenticate, a [UserContext] opens its own connection to the relay in a
// private per-user pool (its "lane"), answers the challenge issued on that
// connection, and from then on routes that user's requests to that relay over
// it. Other users, and anonymous requests, keep using the shared pool.

// authChallengeTimeout bounds how long AuthChallenge waits for a relay to send
// a challenge on a freshly opened connection, when ctx has no sooner deadline.
const authChallengeTimeout = 5 * time.Second

// authOKTimeout bounds how long Authenticate waits for the relay's OK.
const authOKTimeout = 5 * time.Second

// AuthState is what a pool has observed about one relay's NIP-42 status: its
// latest challenge and whether it has been answered on the live connection.
type AuthState struct {
	Relay     string    `json:"relay"`
	Challenge string    `json:"challenge"`
	Authed    bool      `json:"authed"`
	At        time.Time `json:"at"`

	// conn is the connection the challenge arrived on. Authed only holds while
	// that exact connection is the pool's live one: a reconnect gets a new,
	// unauthenticated socket.
	conn *RelayConnection
}

// RouteAuth records a relay's AUTH challenge as it arrived on conn. A new
// challenge clears the authed flag: it must be answered again.
func (mr *MessageRouter) RouteAuth(relayURL, challenge string, conn *RelayConnection) {
	mr.authMu.Lock()
	defer mr.authMu.Unlock()
	st := mr.authStates[relayURL]
	if st == nil {
		st = &AuthState{Relay: relayURL}
		mr.authStates[relayURL] = st
	}
	if challenge != st.Challenge || conn != st.conn {
		st.Authed = false
	}
	st.Challenge = challenge
	st.conn = conn
	st.At = time.Now()

	// Wake anyone waiting for a challenge from this relay.
	if ch, ok := mr.authWait[relayURL]; ok {
		close(ch)
		delete(mr.authWait, relayURL)
	}
}

// markAuthed records that challenge was answered on relayURL's connection. It
// is a no-op if a newer challenge (or connection) has replaced it meanwhile.
func (mr *MessageRouter) markAuthed(relayURL, challenge string) {
	mr.authMu.Lock()
	defer mr.authMu.Unlock()
	if st := mr.authStates[relayURL]; st != nil && st.Challenge == challenge {
		st.Authed = true
	}
}

// AuthChallenge returns a relay's current challenge ("" if none pending).
func (mr *MessageRouter) AuthChallenge(relayURL string) string {
	mr.authMu.RLock()
	defer mr.authMu.RUnlock()
	if st := mr.authStates[relayURL]; st != nil {
		return st.Challenge
	}
	return ""
}

// waitChallenge returns relayURL's challenge if one arrived on conn, else
// waits for one until ctx ends.
func (mr *MessageRouter) waitChallenge(ctx context.Context, relayURL string, conn *RelayConnection) (string, error) {
	for {
		mr.authMu.Lock()
		if st := mr.authStates[relayURL]; st != nil && st.conn == conn && st.Challenge != "" {
			c := st.Challenge
			mr.authMu.Unlock()
			return c, nil
		}
		ch, ok := mr.authWait[relayURL]
		if !ok {
			ch = make(chan struct{})
			mr.authWait[relayURL] = ch
		}
		mr.authMu.Unlock()

		select {
		case <-ch:
		case <-ctx.Done():
			return "", fmt.Errorf("relay %s sent no AUTH challenge: %w", relayURL, ctx.Err())
		}
	}
}

// isAuthedOn reports whether relayURL's challenge was answered on conn.
func (mr *MessageRouter) isAuthedOn(relayURL string, conn *RelayConnection) bool {
	mr.authMu.RLock()
	defer mr.authMu.RUnlock()
	st := mr.authStates[relayURL]
	return st != nil && st.Authed && st.conn == conn
}

// AuthStates returns a snapshot of every relay that has challenged this pool.
func (mr *MessageRouter) AuthStates() []AuthState {
	mr.authMu.RLock()
	defer mr.authMu.RUnlock()
	out := make([]AuthState, 0, len(mr.authStates))
	for _, st := range mr.authStates {
		s := *st
		s.conn = nil
		out = append(out, s)
	}
	return out
}

// RemoveAuth forgets a relay's AUTH state.
func (mr *MessageRouter) RemoveAuth(relayURL string) {
	mr.authMu.Lock()
	delete(mr.authStates, relayURL)
	mr.authMu.Unlock()
}

// ── Pool-level NIP-42 surface ────────────────────────────────────────────────

// AuthRequests returns the relays that have sent this pool an AUTH challenge.
func (rp *RelayPool) AuthRequests() []AuthState { return rp.messageRouter.AuthStates() }

// AuthChallenge returns a relay's pending challenge ("" if none).
func (rp *RelayPool) AuthChallenge(url string) string { return rp.messageRouter.AuthChallenge(url) }

// RemoveAuth forgets a relay's AUTH state.
func (rp *RelayPool) RemoveAuth(url string) { rp.messageRouter.RemoveAuth(url) }

// authedConn returns url's live connection if it has answered an AUTH
// challenge, else nil.
func (rp *RelayPool) authedConn(url string) *RelayConnection {
	rp.mu.RLock()
	conn := rp.connections[url]
	rp.mu.RUnlock()
	if conn == nil || conn.getStatus() != StatusConnected {
		return nil
	}
	if !rp.messageRouter.isAuthedOn(url, conn) {
		return nil
	}
	return conn
}

// ── Client: the shared pool never authenticates ──────────────────────────────

// AuthRequests returns the relays that have challenged the shared pool. These
// are relays that want AUTH; answering one is per user, via
// [UserContext.Authenticate].
func (c *Client) AuthRequests() []AuthState { return c.relayPool.AuthRequests() }

// AuthChallenge returns the challenge a relay sent the shared pool ("" if none).
func (c *Client) AuthChallenge(url string) string { return c.relayPool.AuthChallenge(url) }

// ErrSharedAuth is returned by [Client.SendAuth]. Authenticating the shared
// pool's connection would let every user of the Client read and publish as
// the authenticated pubkey.
var ErrSharedAuth = fmt.Errorf("NIP-42 AUTH on the shared pool is refused: authenticate per user with UserContext.Authenticate")

// SendAuth always fails with [ErrSharedAuth]. It is kept so older callers get
// a clear error instead of silently authenticating a connection every user of
// the Client shares.
//
// Deprecated: use [UserContext.Authenticate] or [UserContext.SignAndAuthenticate].
func (c *Client) SendAuth(url string, signedEvent *nostr.Event) error { return ErrSharedAuth }

// RemoveAuth forgets a relay's challenge on the shared pool.
func (c *Client) RemoveAuth(url string) { c.relayPool.RemoveAuth(url) }

// ── UserContext: per-user authentication ─────────────────────────────────────

// AuthChallenge opens (or reuses) this user's own connection to url and
// returns the AUTH challenge the relay issued on it, waiting for it to arrive.
// Sign a kind-22242 event over it and pass it to [UserContext.Authenticate].
// Relays that only challenge lazily, after a restricted request, are not
// supported yet: this returns an error if no challenge arrives in time.
func (uc *UserContext) AuthChallenge(ctx context.Context, url string) (string, error) {
	norm, ok := normalizeRelayURL(url)
	if !ok {
		return "", fmt.Errorf("invalid relay url: %q", url)
	}
	conn, err := uc.holdLaneConn(norm)
	if err != nil {
		return "", err
	}
	ctx, cancel := withDefaultTimeout(ctx, authChallengeTimeout)
	defer cancel()
	return uc.lane().messageRouter.waitChallenge(ctx, norm, conn)
}

// Authenticate answers url's challenge on this user's own connection with
// signed, a kind-22242 event by this user over the challenge from
// [UserContext.AuthChallenge]. It returns once the relay accepts (or rejects)
// it. From then on this user's requests to url go over the authenticated
// connection; nobody else's do.
func (uc *UserContext) Authenticate(ctx context.Context, url string, signed *nostr.Event) error {
	norm, ok := normalizeRelayURL(url)
	if !ok {
		return fmt.Errorf("invalid relay url: %q", url)
	}
	lane := uc.lane()
	challenge := lane.messageRouter.AuthChallenge(norm)
	if challenge == "" {
		return fmt.Errorf("no AUTH challenge from %s on this user's connection: call AuthChallenge first", norm)
	}
	if err := checkAuthEvent(signed, uc.pubkey, norm, challenge); err != nil {
		return err
	}

	okCh := lane.messageRouter.RegisterOKWaiter(signed.ID, 1)
	defer lane.messageRouter.UnregisterOKWaiter(signed.ID)
	if err := lane.SendMessage(norm, []interface{}{"AUTH", signed}); err != nil {
		return err
	}

	ctx, cancel := withDefaultTimeout(ctx, authOKTimeout)
	defer cancel()
	select {
	case res := <-okCh:
		if !res.Accepted {
			return fmt.Errorf("relay %s rejected AUTH: %s", norm, res.Reason)
		}
	case <-ctx.Done():
		// NIP-42 requires an OK, but some relays don't send one. The socket is
		// this user's alone, so treating it as authed can't leak anything.
		clog().Debug("No OK for AUTH; assuming accepted", "relay", norm, "pubkey", uc.pubkey)
	}
	lane.messageRouter.markAuthed(norm, challenge)
	clog().Info("Authenticated to relay", "relay", norm, "pubkey", uc.pubkey)
	return nil
}

// SignAndAuthenticate fetches url's challenge, signs the kind-22242 response
// with this context's signer, and authenticates. Requires a signer.
func (uc *UserContext) SignAndAuthenticate(ctx context.Context, url string) error {
	norm, ok := normalizeRelayURL(url)
	if !ok {
		return fmt.Errorf("invalid relay url: %q", url)
	}
	challenge, err := uc.AuthChallenge(ctx, norm)
	if err != nil {
		return err
	}
	evt := &nostr.Event{
		Kind:      22242,
		CreatedAt: time.Now().Unix(),
		Tags:      [][]string{{"relay", norm}, {"challenge", challenge}},
	}
	if err := uc.Sign(evt); err != nil {
		return err
	}
	return uc.Authenticate(ctx, norm, evt)
}

// AuthStates lists the relays asking this user for AUTH: every relay that
// challenged this user's own connections (with whether it's answered), plus
// relays that challenged the shared pool and this user hasn't connected to.
func (uc *UserContext) AuthStates() []AuthState {
	lane := uc.lane()
	byRelay := make(map[string]AuthState)
	for _, st := range uc.client.relayPool.AuthRequests() {
		st.Authed = false // the shared pool never authenticates
		byRelay[st.Relay] = st
	}
	for _, st := range lane.AuthRequests() {
		st.Authed = lane.authedConn(st.Relay) != nil
		byRelay[st.Relay] = st
	}
	out := make([]AuthState, 0, len(byRelay))
	for _, st := range byRelay {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Relay < out[j].Relay })
	return out
}

// IsAuthenticated reports whether this user is authenticated to url on a live
// connection.
func (uc *UserContext) IsAuthenticated(url string) bool {
	norm, ok := normalizeRelayURL(url)
	return ok && uc.lane().authedConn(norm) != nil
}

// Deauthenticate closes this user's own connection to url, which is the only
// way to end a NIP-42 session (relays don't support un-authenticating a
// socket). Later requests to url go over the shared pool again.
func (uc *UserContext) Deauthenticate(url string) {
	norm, ok := normalizeRelayURL(url)
	if !ok {
		return
	}
	uc.mu.Lock()
	delete(uc.laneHeld, norm)
	uc.mu.Unlock()
	lane := uc.lane()
	lane.Drop(norm)
	lane.RemoveAuth(norm)
}

// checkAuthEvent validates a kind-22242 AUTH response before it is sent: it
// must be signed by pubkey and answer challenge for relayURL.
func checkAuthEvent(evt *nostr.Event, pubkey, relayURL, challenge string) error {
	if evt == nil {
		return fmt.Errorf("AUTH event is nil")
	}
	if evt.Kind != 22242 {
		return fmt.Errorf("AUTH event must be kind 22242, got %d", evt.Kind)
	}
	if evt.PubKey != pubkey {
		return fmt.Errorf("AUTH event is signed by %s, not this user (%s)", evt.PubKey, pubkey)
	}
	if err := VerifyEvent(evt); err != nil {
		return fmt.Errorf("AUTH event: %w", err)
	}
	var gotChallenge, gotRelay string
	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "challenge":
			gotChallenge = tag[1]
		case "relay":
			gotRelay = tag[1]
		}
	}
	if gotChallenge != challenge {
		return fmt.Errorf("AUTH event answers a different challenge than %s issued on this user's connection", relayURL)
	}
	if norm, ok := normalizeRelayURL(gotRelay); !ok || norm != relayURL {
		return fmt.Errorf("AUTH event relay tag %q does not match %s", gotRelay, relayURL)
	}
	return nil
}

// withDefaultTimeout bounds ctx by d unless it already has a sooner deadline.
func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) < d {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}
