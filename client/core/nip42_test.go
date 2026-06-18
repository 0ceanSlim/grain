package core

import "testing"

func TestMessageRouterAuthState(t *testing.T) {
	mr := NewMessageRouter()
	const relay = "wss://relay.example.com"

	if got := mr.AuthChallenge(relay); got != "" {
		t.Fatalf("expected no challenge initially, got %q", got)
	}

	mr.RouteAuth(relay, "chal-1")
	if got := mr.AuthChallenge(relay); got != "chal-1" {
		t.Errorf("challenge = %q, want chal-1", got)
	}
	states := mr.AuthStates()
	if len(states) != 1 || states[0].Relay != relay || states[0].Authed {
		t.Fatalf("unexpected states: %+v", states)
	}

	mr.MarkAuthed(relay)
	if !mr.AuthStates()[0].Authed {
		t.Error("expected authed after MarkAuthed")
	}

	// A different challenge resets authed (the relay re-challenged).
	mr.RouteAuth(relay, "chal-2")
	if st := mr.AuthStates()[0]; st.Authed || st.Challenge != "chal-2" {
		t.Errorf("new challenge should reset authed: %+v", st)
	}

	// The same challenge again must not reset authed.
	mr.MarkAuthed(relay)
	mr.RouteAuth(relay, "chal-2")
	if !mr.AuthStates()[0].Authed {
		t.Error("identical challenge should not reset authed")
	}

	mr.RemoveAuth(relay)
	if len(mr.AuthStates()) != 0 {
		t.Errorf("expected empty after RemoveAuth, got %d", len(mr.AuthStates()))
	}
}
