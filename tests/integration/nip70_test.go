package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// NIP-70 protected events: an event carrying a single-element `["-"]`
// tag must only be accepted from an authenticated connection whose
// pubkey matches the event author. Tests run against the auth scenario
// relay (port 8186) because auth.relay_url there matches the URL the
// PerformAuth helper passes back; the default relay's relay_url uses
// "localhost" and would mismatch the test client's "127.0.0.1" dial.

var protectedTag = [][]string{{"-"}}

func TestNIP70_RejectUnauthenticated(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer c.Close()

	evt := kp.SignEvent(1, "protected, no auth", protectedTag)
	c.SendEvent(evt)

	ok, reason := c.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected protected event from unauth connection to be rejected, got OK true (%q)", reason)
	}
	if !tests.ContainsAny(reason, "auth-required") {
		t.Errorf("expected auth-required reason, got %q", reason)
	}
}

func TestNIP70_AcceptAuthorAuthenticated(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer c.Close()

	if ok, reason := c.PerformAuth(kp, tests.AuthRelayURL, 3*time.Second); !ok {
		t.Fatalf("auth failed: %q", reason)
	}

	evt := kp.SignEvent(1, "protected, author authed", protectedTag)
	c.SendEvent(evt)

	if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
		t.Fatalf("expected protected event from author to be accepted, got reject (%q)", reason)
	}
}

func TestNIP70_RejectWrongAuthor(t *testing.T) {
	authKp := tests.NewTestKeypair()
	otherKp := tests.NewTestKeypair()

	c := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer c.Close()

	if ok, reason := c.PerformAuth(authKp, tests.AuthRelayURL, 3*time.Second); !ok {
		t.Fatalf("auth failed: %q", reason)
	}

	// Event signed by a different keypair than the authed connection.
	evt := otherKp.SignEvent(1, "protected, impersonation attempt", protectedTag)
	c.SendEvent(evt)

	ok, reason := c.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected protected event with mismatched author to be rejected, got OK true (%q)", reason)
	}
	if !tests.ContainsAny(reason, "restricted", "auth") {
		t.Errorf("expected restricted/auth reason, got %q", reason)
	}
}
