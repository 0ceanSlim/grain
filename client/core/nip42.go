package core

import (
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// NIP-42 AUTH (https://github.com/nostr-protocol/nips/blob/master/42.md).
//
// grain's pool holds the relay socket; the signer lives in the browser. So the
// pool records each relay's AUTH challenge as it arrives, the relay manager
// surfaces the relays that have asked, the browser signs a kind-22242 event, and
// SendAuth relays it back on the same connection. A relay stays "authed" for the
// session once answered; a fresh challenge (new connection / expiry) overwrites
// the stored one so the signer can answer again.

// AuthState is what the pool has observed about one relay's NIP-42 status this
// session: its latest challenge and whether we've answered it.
type AuthState struct {
	Relay     string    `json:"relay"`
	Challenge string    `json:"challenge"`
	Authed    bool      `json:"authed"`
	At        time.Time `json:"at"`
}

// RouteAuth records a relay's AUTH challenge. A new challenge clears the authed
// flag for that relay so the manager prompts a fresh answer.
func (mr *MessageRouter) RouteAuth(relayURL, challenge string) {
	mr.authMu.Lock()
	defer mr.authMu.Unlock()
	st := mr.authStates[relayURL]
	if st == nil {
		st = &AuthState{Relay: relayURL}
		mr.authStates[relayURL] = st
	}
	if challenge != st.Challenge {
		st.Authed = false
	}
	st.Challenge = challenge
	st.At = time.Now()
}

// MarkAuthed records that we've answered a relay's challenge this session.
func (mr *MessageRouter) MarkAuthed(relayURL string) {
	mr.authMu.Lock()
	defer mr.authMu.Unlock()
	if st := mr.authStates[relayURL]; st != nil {
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

// AuthStates returns a snapshot of every relay that has challenged us.
func (mr *MessageRouter) AuthStates() []AuthState {
	mr.authMu.RLock()
	defer mr.authMu.RUnlock()
	out := make([]AuthState, 0, len(mr.authStates))
	for _, st := range mr.authStates {
		out = append(out, *st)
	}
	return out
}

// RemoveAuth forgets a relay's AUTH state (the user revoked trust).
func (mr *MessageRouter) RemoveAuth(relayURL string) {
	mr.authMu.Lock()
	delete(mr.authStates, relayURL)
	mr.authMu.Unlock()
}

// ── Pool-level NIP-42 surface ────────────────────────────────────────────────

// AuthRequests returns the relays that have sent an AUTH challenge this session.
func (rp *RelayPool) AuthRequests() []AuthState { return rp.messageRouter.AuthStates() }

// AuthChallenge returns a relay's pending challenge ("" if none).
func (rp *RelayPool) AuthChallenge(url string) string { return rp.messageRouter.AuthChallenge(url) }

// SendAuth sends a signed NIP-42 auth event (kind 22242) to a relay and marks it
// authed for the session. The connection that issued the challenge must still be
// open (the challenge is bound to it).
func (rp *RelayPool) SendAuth(url string, signedEvent *nostr.Event) error {
	if err := rp.SendMessage(url, []interface{}{"AUTH", signedEvent}); err != nil {
		return err
	}
	rp.messageRouter.MarkAuthed(url)
	return nil
}

// RemoveAuth forgets a relay's AUTH state.
func (rp *RelayPool) RemoveAuth(url string) { rp.messageRouter.RemoveAuth(url) }

// ── Client convenience wrappers ──────────────────────────────────────────────

// AuthRequests returns the relays that have challenged the pool for NIP-42 AUTH.
func (c *Client) AuthRequests() []AuthState { return c.relayPool.AuthRequests() }

// AuthChallenge returns a relay's pending AUTH challenge ("" if none).
func (c *Client) AuthChallenge(url string) string { return c.relayPool.AuthChallenge(url) }

// SendAuth relays a browser-signed kind-22242 event to a relay.
func (c *Client) SendAuth(url string, signedEvent *nostr.Event) error {
	return c.relayPool.SendAuth(url, signedEvent)
}

// RemoveAuth forgets a relay's session AUTH state.
func (c *Client) RemoveAuth(url string) { c.relayPool.RemoveAuth(url) }
