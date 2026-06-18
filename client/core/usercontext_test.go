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

func TestBuildReplyTags(t *testing.T) {
	hasTag := func(tags [][]string, want ...string) bool {
		for _, tg := range tags {
			if len(tg) < len(want) {
				continue
			}
			ok := true
			for i, w := range want {
				if tg[i] != w {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return false
	}
	countP := func(tags [][]string, pk string) int {
		n := 0
		for _, tg := range tags {
			if len(tg) >= 2 && tg[0] == "p" && tg[1] == pk {
				n++
			}
		}
		return n
	}

	// Reply to a top-level note: parent is the root, author gets a p-tag.
	top := &nostr.Event{ID: "P1", PubKey: "AUTHOR", Kind: 1}
	tt := buildReplyTags(top)
	if !hasTag(tt, "e", "P1", "", "root") {
		t.Errorf("top-level reply missing root e-tag: %v", tt)
	}
	if !hasTag(tt, "p", "AUTHOR") {
		t.Errorf("top-level reply missing author p-tag: %v", tt)
	}

	// Reply within a thread: carry the root, mark the parent as reply, notify
	// the whole thread, and don't duplicate the parent author.
	mid := &nostr.Event{
		ID: "P2", PubKey: "BOB", Kind: 1,
		Tags: [][]string{
			{"e", "ROOT", "", "root"},
			{"e", "P1", "", "reply"},
			{"p", "ALICE"},
			{"p", "BOB"}, // parent author already present — must not double
		},
	}
	mt := buildReplyTags(mid)
	if !hasTag(mt, "e", "ROOT", "", "root") {
		t.Errorf("threaded reply missing root: %v", mt)
	}
	if !hasTag(mt, "e", "P2", "", "reply") {
		t.Errorf("threaded reply missing reply marker: %v", mt)
	}
	if !hasTag(mt, "p", "ALICE") || countP(mt, "BOB") != 1 {
		t.Errorf("threaded reply p-tags wrong (want ALICE + single BOB): %v", mt)
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
