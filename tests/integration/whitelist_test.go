package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-whitelist (port 8185). whitelist-rules.yml allows
// only the deterministic pubkey derived from tests.WhitelistSeed, and only
// kinds 0 and 1.

func TestWhitelist_AllowedPubkey(t *testing.T) {
	kp := tests.NewDeterministicKeypair(tests.WhitelistSeed)
	client := tests.NewTestClientAt(t, tests.WhitelistRelayURL)
	defer client.Close()

	evt := kp.SignEvent(1, "hello from whitelisted pubkey", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if !ok {
		t.Fatalf("expected whitelisted pubkey event to be accepted, got %q", reason)
	}
}

func TestWhitelist_DeniedPubkey(t *testing.T) {
	kp := tests.NewTestKeypair() // random, not whitelisted
	client := tests.NewTestClientAt(t, tests.WhitelistRelayURL)
	defer client.Close()

	evt := kp.SignEvent(1, "random pubkey should be rejected", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected non-whitelisted pubkey to be rejected")
	}
	if !strings.Contains(reason, "not allowed") {
		t.Fatalf("expected whitelist reject, got %q", reason)
	}
}

func TestWhitelist_DeniedKind(t *testing.T) {
	kp := tests.NewDeterministicKeypair(tests.WhitelistSeed)
	client := tests.NewTestClientAt(t, tests.WhitelistRelayURL)
	defer client.Close()

	// Kind 7 is not in the whitelisted kinds list (only 0 and 1).
	evt := kp.SignEvent(7, "+", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected disallowed kind to be rejected")
	}
	if !strings.Contains(reason, "not allowed") {
		t.Fatalf("expected kind-whitelist reject, got %q", reason)
	}
}
