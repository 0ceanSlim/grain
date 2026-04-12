package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-blacklist (port 8184).
// blacklist-rules.yml contents:
//   permanent_ban_words: ["permaword"]
//   temp_ban_words:      ["tempword"]
//   max_temp_bans:       2
//   temp_ban_duration:   3 seconds

func TestBlacklist_PermanentBannedWord(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.BlacklistRelayURL)
	defer client.Close()

	evt := kp.SignEvent(1, "this contains permaword and should be blocked", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected permanent-ban-word event to be rejected")
	}
	if !strings.Contains(reason, "blocked:") {
		t.Fatalf("expected blocked reject, got %q", reason)
	}
}

func TestBlacklist_TempBannedWord(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.BlacklistRelayURL)
	defer client.Close()

	// First event containing tempword should be rejected.
	evt1 := kp.SignEvent(1, "contains tempword", nil)
	client.SendEvent(evt1)
	ok, reason := client.ExpectOK(evt1.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected temp-ban-word event to be rejected")
	}
	if !strings.Contains(reason, "blocked:") {
		t.Fatalf("expected blocked reject, got %q", reason)
	}

	// A clean event from the same pubkey should now be temp-blocked until the
	// 3-second ban expires.
	evt2 := kp.SignEvent(1, "clean content but pubkey is temp-banned", nil)
	client.SendEvent(evt2)
	ok2, reason2 := client.ExpectOK(evt2.ID, 3*time.Second)
	if ok2 {
		t.Fatalf("expected follow-up from temp-banned pubkey to be rejected")
	}
	if !tests.ContainsAny(reason2,
		"temporarily blacklisted",
		"temporarily banned",
		"blocked:",
	) {
		t.Fatalf("expected temp-ban reject, got %q", reason2)
	}
}

func TestBlacklist_PermanentBanEscalation(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.BlacklistRelayURL)
	defer client.Close()

	// max_temp_bans = 2: need 3 temp-ban-word events to escalate.
	for i := 0; i < 3; i++ {
		evt := kp.SignEvent(1, "tempword trigger", nil)
		client.SendEvent(evt)
		client.ExpectOK(evt.ID, 3*time.Second)
	}

	// Wait past temp_ban_duration so we know the next rejection is coming
	// from the permanent escalation path, not lingering temp state.
	time.Sleep(4 * time.Second)

	evt := kp.SignEvent(1, "even clean content is blocked after escalation", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected escalated pubkey to be permanently blocked")
	}
	if !strings.Contains(reason, "permanently banned") {
		t.Fatalf("expected 'permanently banned' reject, got %q", reason)
	}
}
