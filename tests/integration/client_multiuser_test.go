package integration

import (
	"context"
	"testing"
	"time"

	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/tests"
)

// Regression, end to end against a real grain relay: grain's client library
// used to send NIP-42 AUTH on the connection every user of a Client shares, so
// once Alice authenticated, the relay served Bob (and anonymous requests) as
// Alice — including the NIP-17 gift wraps it only serves to their recipient.
// Now each user authenticates on their own connection and only their own
// requests use it.
func TestClientLibrary_GiftWrapsServedOnlyToAuthenticatedRecipient(t *testing.T) {
	c := core.NewClient(core.DefaultConfig())
	defer c.Close()

	newUser := func() *core.UserContext {
		signer, err := core.NewEventSignerFromRandom()
		if err != nil {
			t.Fatal(err)
		}
		return c.NewUserContext(signer.PublicKey(), core.WithSigner(signer))
	}
	alice, bob := newUser(), newUser()
	defer alice.Close()
	defer bob.Close()

	// Warm the shared connection first: it exists before anyone authenticates.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.QueryEvents(ctx, nostr.Filter{Kinds: []int{1}, Limit: intPtr(1)}, []string{tests.AuthRelayURL}, core.WithTimeout(3*time.Second))

	if err := alice.SignAndAuthenticate(ctx, tests.AuthRelayURL); err != nil {
		t.Fatalf("alice auth: %v", err)
	}
	if err := bob.SignAndAuthenticate(ctx, tests.AuthRelayURL); err != nil {
		t.Fatalf("bob auth: %v", err)
	}

	// A gift wrap for Alice, from an ephemeral key as NIP-17 does.
	wrapper := tests.NewTestKeypair()
	pub := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer pub.Close()
	wrap := wrapper.SignEvent(1059, "sealed", [][]string{{"p", alice.PublicKey()}})
	pub.SendMessage([]interface{}{"EVENT", wrap})
	if ok, reason := pub.ExpectOK(wrap.ID, 5*time.Second); !ok {
		t.Fatalf("publish gift wrap: %s", reason)
	}

	filter := nostr.Filter{Kinds: []int{1059}, Tags: map[string][]string{"p": {alice.PublicKey()}}}
	query := func(q func(context.Context, nostr.Filter, []string, ...core.StreamOption) []*nostr.Event) []*nostr.Event {
		qctx, qcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer qcancel()
		return q(qctx, filter, []string{tests.AuthRelayURL}, core.WithTimeout(4*time.Second))
	}

	// Alice gets it once committed (poll: the relay commits asynchronously).
	deadline := time.Now().Add(15 * time.Second)
	for {
		if evs := query(alice.QueryEvents); len(evs) == 1 && evs[0].ID == wrap.ID {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("alice never received her own gift wrap")
		}
		time.Sleep(200 * time.Millisecond)
	}

	// It's committed now, so these answers are meaningful.
	if evs := query(bob.QueryEvents); len(evs) != 0 {
		t.Fatalf("bob (authenticated as himself) received alice's gift wrap: %d events", len(evs))
	}
	if evs := query(c.QueryEvents); len(evs) != 0 {
		t.Fatalf("an anonymous query received alice's gift wrap: %d events", len(evs))
	}

	// After Alice deauthenticates, not even her requests are served as her.
	alice.Deauthenticate(tests.AuthRelayURL)
	if evs := query(alice.QueryEvents); len(evs) != 0 {
		t.Fatalf("alice still served as herself after Deauthenticate: %d events", len(evs))
	}
}

func intPtr(n int) *int { return &n }
