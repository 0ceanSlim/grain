package core

import (
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

// Exercise EventSigner purely through the Signer seam: sign an event, then
// verify it — the round-trip a library consumer relies on.
func TestEventSignerSatisfiesSigner(t *testing.T) {
	es, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatalf("NewEventSignerFromRandom: %v", err)
	}

	var s Signer = es // compile-time + runtime: EventSigner is a Signer

	if len(s.PublicKey()) != 64 {
		t.Fatalf("PublicKey() = %q, want 64 hex chars", s.PublicKey())
	}

	evt := &nostr.Event{Kind: 1, Content: "hello", CreatedAt: 1700000000}
	if err := s.SignEvent(evt); err != nil {
		t.Fatalf("SignEvent: %v", err)
	}
	if evt.PubKey != s.PublicKey() {
		t.Errorf("SignEvent left PubKey = %q, want %q", evt.PubKey, s.PublicKey())
	}
	if evt.ID == "" || evt.Sig == "" {
		t.Fatalf("SignEvent left ID=%q Sig=%q", evt.ID, evt.Sig)
	}
	if !VerifyEventSignature(evt) {
		t.Error("VerifyEventSignature failed on a freshly signed event")
	}
}
