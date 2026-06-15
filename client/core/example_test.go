package core_test

// These examples are the compile-checked public-API surface of the importable
// library: they live in an external test package (core_test) and import the
// library the way a downstream consumer does, so the API can't silently drift
// without breaking the build (CI runs `go test ./...`). They demonstrate shape,
// not output, so they compile but do not run (no `// Output:`).

import (
	"context"
	"fmt"
	"time"

	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
)

// Read-only: fetch a user's recent notes from their outbox relays. No signer is
// attached, so the context can read but not publish.
func ExampleClient_readOnly() {
	client := core.NewClient(core.DefaultConfig())
	uc := client.NewUserContext("author-pubkey-hex")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	notes := uc.FetchNotes(ctx, "author-pubkey-hex", core.WithLimit(20))
	fmt.Printf("fetched %d notes\n", len(notes))
}

// Publishing: attach a local-key signer and post an outbox-routed reply.
func ExampleUserContext_Reply() {
	client := core.NewClient(core.DefaultConfig())

	signer, err := core.NewEventSigner("64-char-hex-private-key")
	if err != nil {
		return
	}
	uc := client.NewUserContext(signer.PublicKey(), core.WithSigner(signer))

	parent := &nostr.Event{ID: "parent-id", PubKey: "parent-author", Kind: 1}
	reply, results, err := uc.Reply(parent, "well said!")
	if err != nil {
		return
	}
	fmt.Printf("published %s to %d relays\n", reply.ID, len(results))
}

// Streaming: a live feed of kind-1 notes from a single relay (e.g. grain's own).
func ExampleClient_StreamEvents() {
	client := core.NewClient(core.DefaultConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	feed := client.StreamEvents(ctx,
		nostr.Filter{Kinds: []int{1}},
		[]string{"wss://relay.example.com"},
		core.WithLive(), core.WithLimit(50))

	for ev := range feed {
		fmt.Println(ev.ID)
	}
}

// Inspecting and editing the role-tagged session relay configuration.
func ExampleSessionRelays() {
	client := core.NewClient(core.DefaultConfig())
	uc := client.NewUserContext("pubkey-hex")

	uc.Relays().Set("wss://relay.example.com", core.RoleOutbox|core.RoleInbox)
	uc.Relays().Add("wss://dm.example.com", core.RoleDMInbox)

	for _, url := range uc.Relays().ByRole(core.RoleOutbox) {
		fmt.Println("outbox:", url)
	}
}
