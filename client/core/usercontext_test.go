package core

import (
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestNewUserContext(t *testing.T) {
	c := NewClient(DefaultConfig())
	uc := c.NewUserContext("abc123")
	if uc.PublicKey() != "abc123" {
		t.Errorf("PublicKey() = %q", uc.PublicKey())
	}
	if uc.Client() != c {
		t.Error("Client() should return the constructing client")
	}
	if uc.Signer() != nil {
		t.Error("a context built without WithSigner should be read-only")
	}
	if uc.Relays() == nil {
		t.Error("Relays() must not be nil")
	}
}

func TestUserContextSign(t *testing.T) {
	c := NewClient(DefaultConfig())

	// Read-only context: Sign must refuse.
	ro := c.NewUserContext("deadbeef")
	if err := ro.Sign(&nostr.Event{Kind: 1}); err == nil {
		t.Error("read-only context should fail to Sign")
	}

	es, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatalf("NewEventSignerFromRandom: %v", err)
	}

	// Signer pubkey must match the context pubkey.
	mismatch := c.NewUserContext("not-the-signers-key", WithSigner(es))
	if err := mismatch.Sign(&nostr.Event{Kind: 1}); err == nil {
		t.Error("Sign should reject a signer whose pubkey != context pubkey")
	}

	// Matching context: signs, stamps pubkey, verifies.
	uc := c.NewUserContext(es.PublicKey(), WithSigner(es))
	evt := &nostr.Event{Kind: 1, Content: "hi", CreatedAt: 1700000000}
	if err := uc.Sign(evt); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if evt.PubKey != es.PublicKey() {
		t.Errorf("Sign left PubKey = %q", evt.PubKey)
	}
	if !VerifyEventSignature(evt) {
		t.Error("signed event failed verification")
	}
}

func TestSessionRelays(t *testing.T) {
	sr := newSessionRelays()
	sr.Set("wss://a", RoleOutbox|RoleInbox)
	sr.Set("wss://b", RoleInbox)
	sr.Add("wss://b", RoleDMInbox)

	if got := sr.Get("wss://a"); got != RoleOutbox|RoleInbox {
		t.Errorf("Get(a) = %v", got)
	}
	if got := sr.Get("wss://b"); got != RoleInbox|RoleDMInbox {
		t.Errorf("Get(b) = %v", got)
	}
	if inboxes := sr.ByRole(RoleInbox); len(inboxes) != 2 || inboxes[0] != "wss://a" || inboxes[1] != "wss://b" {
		t.Errorf("ByRole(Inbox) = %v (want sorted [a b])", inboxes)
	}
	if out := sr.ByRole(RoleOutbox); len(out) != 1 || out[0] != "wss://a" {
		t.Errorf("ByRole(Outbox) = %v", out)
	}

	sr.Set("wss://a", 0) // setting zero removes
	if got := sr.Get("wss://a"); got != 0 {
		t.Errorf("Set(a, 0) should remove; Get = %v", got)
	}
	sr.Remove("wss://b")
	if len(sr.All()) != 0 {
		t.Errorf("All() = %v, want empty", sr.All())
	}
}
