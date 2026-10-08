package core

import (
	"context"
	"testing"
	"time"
)

func TestMessageRouterAuthState(t *testing.T) {
	mr := NewMessageRouter()
	const relay = "wss://relay.example.com"
	conn := &RelayConnection{URL: relay, Status: StatusConnected}

	if got := mr.AuthChallenge(relay); got != "" {
		t.Fatalf("expected no challenge initially, got %q", got)
	}

	mr.RouteAuth(relay, "chal-1", conn)
	if got := mr.AuthChallenge(relay); got != "chal-1" {
		t.Errorf("challenge = %q, want chal-1", got)
	}
	states := mr.AuthStates()
	if len(states) != 1 || states[0].Relay != relay || states[0].Authed {
		t.Fatalf("unexpected states: %+v", states)
	}

	mr.markAuthed(relay, "chal-1")
	if !mr.AuthStates()[0].Authed || !mr.isAuthedOn(relay, conn) {
		t.Error("expected authed after markAuthed")
	}

	// A different challenge resets authed (the relay re-challenged).
	mr.RouteAuth(relay, "chal-2", conn)
	if st := mr.AuthStates()[0]; st.Authed || st.Challenge != "chal-2" {
		t.Errorf("new challenge should reset authed: %+v", st)
	}

	// Answering a superseded challenge doesn't count.
	mr.markAuthed(relay, "chal-1")
	if mr.AuthStates()[0].Authed {
		t.Error("answer to a stale challenge marked authed")
	}

	// The same challenge again must not reset authed.
	mr.markAuthed(relay, "chal-2")
	mr.RouteAuth(relay, "chal-2", conn)
	if !mr.AuthStates()[0].Authed {
		t.Error("identical challenge should not reset authed")
	}

	// Authed is bound to the connection: another one isn't authed.
	other := &RelayConnection{URL: relay, Status: StatusConnected}
	if mr.isAuthedOn(relay, other) {
		t.Error("authed state leaked to a different connection")
	}
	mr.RouteAuth(relay, "chal-2", other)
	if mr.AuthStates()[0].Authed {
		t.Error("same challenge on a new connection should reset authed")
	}

	mr.RemoveAuth(relay)
	if len(mr.AuthStates()) != 0 {
		t.Errorf("expected empty after RemoveAuth, got %d", len(mr.AuthStates()))
	}
}

func TestMessageRouterWaitChallenge(t *testing.T) {
	mr := NewMessageRouter()
	const relay = "wss://relay.example.com"
	conn := &RelayConnection{URL: relay}

	go func() {
		time.Sleep(20 * time.Millisecond)
		mr.RouteAuth(relay, "late", conn)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := mr.waitChallenge(ctx, relay, conn)
	if err != nil || got != "late" {
		t.Fatalf("waitChallenge = %q, %v; want late", got, err)
	}

	// A challenge from a different connection doesn't satisfy the wait.
	short, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if _, err := mr.waitChallenge(short, relay, &RelayConnection{URL: relay}); err == nil {
		t.Fatal("accepted a challenge issued on another connection")
	}
}
