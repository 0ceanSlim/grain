package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-timecheck (port 8187) with a ±60s drift window.

func TestTimeCheck_TooOld(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.TimeCheckRelayURL)
	defer client.Close()

	evt := kp.SignEventAt(1, "from the past", nil, time.Now().Add(-5*time.Minute).Unix())
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected too-old event to be rejected")
	}
	if !strings.Contains(reason, "invalid: event created_at timestamp is out of allowed range") {
		t.Fatalf("expected timestamp reject, got %q", reason)
	}
}

func TestTimeCheck_TooNew(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.TimeCheckRelayURL)
	defer client.Close()

	evt := kp.SignEventAt(1, "from the future", nil, time.Now().Add(5*time.Minute).Unix())
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if ok {
		t.Fatalf("expected too-new event to be rejected")
	}
	if !strings.Contains(reason, "invalid: event created_at timestamp is out of allowed range") {
		t.Fatalf("expected timestamp reject, got %q", reason)
	}
}

func TestTimeCheck_WithinRange(t *testing.T) {
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.TimeCheckRelayURL)
	defer client.Close()

	evt := kp.SignEventAt(1, "just right", nil, time.Now().Unix())
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 3*time.Second)
	if !ok {
		t.Fatalf("expected in-range event to be accepted, got %q", reason)
	}
}
