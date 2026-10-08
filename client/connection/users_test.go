package connection

import (
	"testing"

	"github.com/0ceanslim/grain/client/core"
)

func TestUserFor(t *testing.T) {
	coreClient = core.NewClient(core.DefaultConfig())
	defer func() { coreClient = nil }()

	a := UserFor("alice")
	if a == nil || UserFor("alice") != a {
		t.Fatal("UserFor should return one context per pubkey")
	}
	if UserFor("bob") == a {
		t.Fatal("two users share a context")
	}
	if UserFor("") != nil {
		t.Fatal("empty pubkey should get no context")
	}

	ReleaseUser("alice")
	if err := a.HoldRelays(nil); err == nil {
		t.Fatal("released context is still open")
	}
	if UserFor("alice") == a {
		t.Fatal("released context still registered")
	}

	// A replaced core client (config reload) gets fresh contexts.
	b := UserFor("bob")
	coreClient = core.NewClient(core.DefaultConfig())
	if nb := UserFor("bob"); nb == b || nb.Client() != coreClient {
		t.Fatal("context from the old core client reused after replacement")
	}
	if err := b.HoldRelays(nil); err == nil {
		t.Fatal("context on the old core client left open")
	}

	coreClient = nil
	if UserFor("bob") != nil {
		t.Fatal("no core client should mean no context")
	}
}
