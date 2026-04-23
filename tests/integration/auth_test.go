package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-auth (port 8186), which is configured with
// auth.required = false and auth.relay_url = "ws://localhost:8186".
// The relay proactively sends an AUTH challenge on connect; tests read
// that challenge and use it to construct their AUTH events.

func TestAuth_ValidAuthAccepted(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer client.Close()

	challenge := client.ExpectAuthChallenge(3 * time.Second)
	if challenge == "" {
		t.Fatal("did not receive AUTH challenge on connect")
	}

	tags := [][]string{
		{"relay", "ws://localhost:8186"},
		{"challenge", challenge},
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

	challenge := client.ExpectAuthChallenge(3 * time.Second)
	if challenge == "" {
		t.Fatal("did not receive AUTH challenge on connect")
	}

	tags := [][]string{
		{"relay", "ws://wrong-relay.example.com"},
		{"challenge", challenge},
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

	challenge := client.ExpectAuthChallenge(3 * time.Second)
	if challenge == "" {
		t.Fatal("did not receive AUTH challenge on connect")
	}

	// Kind 1 is not 22242; AUTH must reject.
	tags := [][]string{
		{"relay", "ws://localhost:8186"},
		{"challenge", challenge},
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
