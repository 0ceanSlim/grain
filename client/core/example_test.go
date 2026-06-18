package core_test

// These examples are the compile-checked public-API surface of the importable
// library: they live in an external test package (core_test) and import the
// library the way a downstream consumer does, so the API can't silently drift
// without breaking the build (CI runs `go test ./...`). They demonstrate shape,
// not output, so they compile but do not run (no `// Output:`).

import (
	"context"
	"fmt"
	"log/slog"
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
	reply, results, err := uc.Reply(context.Background(), parent, "well said!")
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

// Resolving a user's media servers (Blossom + NIP-96) before an upload. HasAny
// is the "open the picker vs. prompt to set some up" decision.
func ExampleClient_ResolveMediaServers() {
	client := core.NewClient(core.DefaultConfig())

	ms := client.ResolveMediaServers("author-pubkey-hex")
	if !ms.HasAny() {
		fmt.Println("no media servers configured")
		return
	}
	fmt.Printf("blossom: %v\n", ms.Blossom) // primary first
}

// Reading a user's relay lists: NIP-65 (with read/write markers) plus the
// NIP-51/37 mailbox lists.
func ExampleClient_FetchUserRelayLists() {
	client := core.NewClient(core.DefaultConfig())

	lists := client.FetchUserRelayLists("author-pubkey-hex")
	for _, e := range lists.NIP65 {
		fmt.Println(e.URL, "read:", e.Read, "write:", e.Write)
	}
	fmt.Println("dm relays:", lists.DM)
}

// Writing a relay list: assemble an UNSIGNED 10002 from the desired entries,
// then sign + outbox-route + publish it with the user context.
func ExampleAssembleRelayListEvent() {
	client := core.NewClient(core.DefaultConfig())
	signer, err := core.NewEventSigner("64-char-hex-private-key")
	if err != nil {
		return
	}
	uc := client.NewUserContext(signer.PublicKey(), core.WithSigner(signer))

	entries := []core.RelayListEntry{
		{URL: "wss://out.example.com", Write: true},              // outbox only
		{URL: "wss://in.example.com", Read: true},                // inbox only
		{URL: "wss://both.example.com", Read: true, Write: true}, // both (unmarked)
	}
	unsigned, err := core.AssembleRelayListEvent(nil, 10002, uc.PublicKey(), entries)
	if err != nil {
		return
	}
	results, err := uc.SignAndPublish(context.Background(), unsigned)
	if err != nil {
		return
	}
	fmt.Printf("published to %d relays\n", len(results))
}

// NIP-44 v2 conversation encryption with the built-in signer (the same key
// holder decrypts).
func ExampleEventSigner_NIP44Encrypt() {
	signer, err := core.NewEventSigner("64-char-hex-private-key")
	if err != nil {
		return
	}
	ciphertext, err := signer.NIP44Encrypt("peer-pubkey-hex", "hello")
	if err != nil {
		return
	}
	plaintext, err := signer.NIP44Decrypt("peer-pubkey-hex", ciphertext)
	if err != nil {
		return
	}
	fmt.Println(plaintext)
}

// Answering a relay's NIP-42 AUTH challenge: build + sign a kind-22242 event and
// forward it on the challenged connection.
func ExampleClient_SendAuth() {
	client := core.NewClient(core.DefaultConfig())
	signer, err := core.NewEventSigner("64-char-hex-private-key")
	if err != nil {
		return
	}
	uc := client.NewUserContext(signer.PublicKey(), core.WithSigner(signer))

	for _, req := range client.AuthRequests() {
		if req.Authed {
			continue // already answered this session
		}
		ev := &nostr.Event{
			Kind: 22242,
			Tags: [][]string{
				{"relay", req.Relay},
				{"challenge", req.Challenge},
			},
		}
		if err := uc.Sign(ev); err != nil {
			continue
		}
		if err := client.SendAuth(req.Relay, ev); err != nil {
			continue
		}
	}
}

// Browsing known relays: the live set, NIP-11 metadata, and TCP latency.
func ExampleClient_KnownRelays() {
	client := core.NewClient(core.DefaultConfig())

	known := client.KnownRelays()
	pings := client.PingRelays(known) // map[url]ms, concurrent

	for _, url := range known {
		name := url
		if info := client.FetchRelayInfo(url); info != nil && info.Name != "" {
			name = info.Name // NIP-11, TTL-cached; nil if the relay serves none
		}
		fmt.Printf("%s — %dms\n", name, pings[url])
	}
}

// Stamping grain's NIP-89 client tag on an event before signing (any foreign
// client tag is stripped first; pass false to strip without re-adding).
func ExampleApplyClientTag() {
	ev := &nostr.Event{Kind: 1, Content: "gm"}
	core.ApplyClientTag(ev, true, "grain")
	fmt.Println(len(ev.Tags))
}

// Replacing grain's logging so the library writes through a consumer's logger.
// Any *slog.Logger satisfies core.Logger; a consumer can also implement the four
// methods (Debug/Info/Warn/Error) over their own logging stack.
func ExampleSetLogger() {
	core.SetLogger(slog.Default())
}

// exampleStore is a minimal RelayListStore — a pubkey -> *UserRelays map. The
// directory owns the TTL and single-flight logic, so a store only needs to be a
// correct key-value map; a real one would persist to a database.
type exampleStore struct{ m map[string]*core.UserRelays }

func (s exampleStore) Get(pk string) (*core.UserRelays, bool) { ur, ok := s.m[pk]; return ur, ok }
func (s exampleStore) Set(pk string, ur *core.UserRelays)     { s.m[pk] = ur }
func (s exampleStore) Delete(pk string)                       { delete(s.m, pk) }
func (s exampleStore) Range(fn func(string, *core.UserRelays) bool) {
	for k, v := range s.m {
		if !fn(k, v) {
			return
		}
	}
}

// Backing the relay directory with a custom store (e.g. a database) instead of
// the default in-memory cache.
func ExampleConfig_relayListStore() {
	cfg := core.DefaultConfig()
	cfg.RelayListStore = exampleStore{m: map[string]*core.UserRelays{}}
	_ = core.NewClient(cfg)
}
