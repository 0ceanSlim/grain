package handlers

import (
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
	"golang.org/x/net/websocket"
)

// noopClient is a minimal nostr.ClientInterface implementation used purely as
// a unique map key in the auth-state tests. Methods are stubs — these tests
// only exercise the per-connection auth maps in auth.go. A pointer
// (&noopClient{}) is used as the key so each instance has a distinct identity
// (empty structs compare equal by value, which would collide as map keys).
type noopClient struct{}

func (noopClient) SendMessage(interface{})                          {}
func (noopClient) SendMessageBlocking(interface{}) error            { return nil }
func (noopClient) GetWS() *websocket.Conn                           { return nil }
func (noopClient) GetSubscriptions() map[string][]nostr.Filter      { return nil }
func (noopClient) SetSubscription(string, []nostr.Filter)           {}
func (noopClient) DeleteSubscription(string)                        {}
func (noopClient) SubscriptionCount() int                           { return 0 }
func (noopClient) ForEachSubscription(func(string, []nostr.Filter)) {}
func (noopClient) CloseClient()                                     {}
func (noopClient) IsConnected() bool                                { return true }
func (noopClient) AllowReq() (bool, string)                         { return true, "" }
func (noopClient) AllowEvent(int, string) (bool, string)            { return true, "" }

// TestCleanupClientRemovesAuthState is the regression test for #92: a
// disconnected client must leave no entry behind in either the challenge or
// the auth-session map, otherwise the client object graph is pinned forever.
func TestCleanupClientRemovesAuthState(t *testing.T) {
	c := &noopClient{}

	SetChallengeForConnection(c, "challenge-123")
	SetAuthenticated(c, "pubkeyhex")

	// Preconditions: state is actually present.
	if got := GetChallengeForConnection(c); got != "challenge-123" {
		t.Fatalf("precondition failed: challenge = %q, want %q", got, "challenge-123")
	}
	if !IsAuthenticated(c) {
		t.Fatal("precondition failed: client should be authenticated")
	}

	CleanupClient(c)

	// Check the maps directly (in-package) — GetX accessors return zero
	// values for both "absent" and "present but empty", so membership is
	// the only way to prove the key was actually deleted.
	authMu.Lock()
	_, challengePresent := challenges[c]
	_, sessionPresent := authSessions[c]
	authMu.Unlock()

	if challengePresent {
		t.Error("challenge entry still present after CleanupClient")
	}
	if sessionPresent {
		t.Error("authSession entry still present after CleanupClient")
	}
	if IsAuthenticated(c) {
		t.Error("client still reports authenticated after CleanupClient")
	}
}

// TestCleanupClientIdempotent verifies that cleaning up a client that was
// never registered (or is cleaned twice) is a harmless no-op, not a panic.
// Both cleanup paths in server/client.go can fire for the same connection.
func TestCleanupClientIdempotent(t *testing.T) {
	c := &noopClient{}
	CleanupClient(c) // never registered
	CleanupClient(c) // double cleanup
}
