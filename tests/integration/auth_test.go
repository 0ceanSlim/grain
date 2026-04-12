package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-auth (port 8186) with auth.enabled = true and
// auth.relay_url = "ws://localhost:8186".
//
// NOTE: the current grain server does not push a proactive AUTH challenge
// frame on connect — it only *validates* AUTH messages that clients send.
// GetChallengeForConnection returns "" for any unknown pubkey, so an AUTH
// event with an empty challenge tag will match. These tests exercise the
// relay-URL and kind checks in VerifyAuthEvent, which are the parts that
// matter for NIP-42 correctness.

func TestAuth_ValidAuthAccepted(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer client.Close()

	// Empty-string challenge matches the map default, relay URL matches.
	tags := [][]string{
		{"relay", "ws://localhost:8186"},
		{"challenge", ""},
	}
	evt := kp.SignEvent(22242, "", tags)
	client.SendMessage([]interface{}{"AUTH", evt})
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if !ok {
		t.Fatalf("expected AUTH to be accepted, got %q", reason)
	}
}

func TestAuth_WrongRelayURL(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer client.Close()

	tags := [][]string{
		{"relay", "ws://wrong-relay.example.com"},
		{"challenge", ""},
	}
	evt := kp.SignEvent(22242, "", tags)
	client.SendMessage([]interface{}{"AUTH", evt})
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected AUTH with wrong relay URL to be rejected")
	}
	if !strings.Contains(reason, "relay URL does not match") {
		t.Fatalf("expected relay URL reject, got %q", reason)
	}
}

func TestAuth_WrongKind(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer client.Close()

	// Kind 1 is not 22242; AUTH must reject.
	tags := [][]string{
		{"relay", "ws://localhost:8186"},
		{"challenge", ""},
	}
	evt := kp.SignEvent(1, "", tags)
	client.SendMessage([]interface{}{"AUTH", evt})
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected AUTH with wrong kind to be rejected")
	}
	if !strings.Contains(reason, "kind must be 22242") {
		t.Fatalf("expected kind reject, got %q", reason)
	}
}
