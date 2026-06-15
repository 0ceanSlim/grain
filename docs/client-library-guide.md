# grain client library guide (0.8.0)

grain's `client/core` package is an **importable, outbox-model Nostr client
engine** in Go. A downstream app builds its own client on it — it does not
reimplement relay routing. The engine owns a shared connection pool, resolves
each user's relay lists, and routes every operation to the right relays under the
[outbox model](https://mikedilger.com/gossip-model/): you read a user's notes
from *their* outbox, and a reply you publish reaches the *parent author's* inbox.

```
import "github.com/0ceanslim/grain/client/core"
```

`client/core` is pure Go (no cgo, no web/HTTP dependencies). grain's own
`client/api`, `client/session`, `client/connection`, and `client/data` packages
are the **reference consumer** of this surface — a worked example of how to build
on it, not part of its import contract.

> **Status (0.8.0, pre-1.0).** The surface below is stable enough to build on,
> with two known gaps: the network methods don't take a `context.Context` on the
> *existing* (pre-0.8) methods yet — the newer `StreamEvents`/`QueryEvents` do —
> and `PublishDM` (gift-wrapped NIP-17) is deferred until NIP-44 encryption
> lands. See [the design doc](design/outbox-relay-pool.md) §11.

---

## Quick start

### Read-only: fetch a user's notes

```go
client := core.NewClient(core.DefaultConfig())     // owns the shared pool
uc := client.NewUserContext(authorHex)             // no signer → read-only

ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
defer cancel()

notes := uc.FetchNotes(ctx, authorHex, core.WithLimit(20)) // → author's outbox
```

`FetchNotes` resolves the author's NIP-65 outbox relays and queries them
concurrently, returning the kind-1 notes it collects. It is best-effort:
per-relay failures are logged, so an empty slice means "none found".

### Publishing: sign and broadcast a reply

```go
signer, _ := core.NewEventSigner(privateKeyHex)            // local-key Signer
uc := client.NewUserContext(signer.PublicKey(), core.WithSigner(signer))

parent := /* the event being replied to */
reply, results, err := uc.Reply(parent, "well said!")
```

`Reply` builds a NIP-10 reply (thread root + parent markers, thread-wide `p`
tags), signs it as the context user, and publishes it under the outbox model —
the author's own outbox **plus** the parent author's inbox — returning the
per-relay [`BroadcastResult`](#broadcastresult)s.

To publish an event you've built yourself: `uc.SignAndPublish(event)` (sign +
route + broadcast), or `uc.Sign(event)` then `uc.Publish(event)` separately.

---

## The outbox model & routing

Routing is automatic: you name the intent, the engine picks the relays.

| Operation | Routes to |
|---|---|
| Read a user's notes | their **outbox** (NIP-65 write), falling back to the indexers |
| Read profile / relay lists | the **indexers** (+ the user's cached outbox) |
| Publish a note | the author's **outbox** ∪ each p-tagged recipient's **inbox** |
| Publish profile / relay list | the author's outbox ∪ the **indexers** |

Inspect a routing decision without performing it:

```go
relays := client.Route(core.OpFetchNotes, authorHex) // == client.RouteFetch(authorHex)
relays = client.RoutePublish(event)                  // per-event (recipients matter)
ur := client.ResolveRelays(authorHex)                // *UserRelays: Outbox/Inbox/DMInbox
```

**Fixed-relay override (opt-out).** For a pinned single-/few-relay client,
`uc.PinFixedRelays(read, write)` routes everything to a fixed set and **disables
the outbox model** (replies stop reaching others' inboxes). Off by default and
discouraged; clear with `uc.ClearFixedRelays()`.

---

## The role model

Every relay serves one or more **roles**, a [`Role`](#role) bitmask split into
three classes:

- **Per-target, event-derived** — resolved from the *target* user's lists:
  `RoleOutbox`, `RoleInbox` (NIP-65 10002), `RoleDMInbox` (NIP-17 10050).
- **Self-only, event-derived** — the logged-in user's own lists: `RoleSearch`
  (10007), `RoleBlocked` (10006), `RoleFavorite` (10012), `RolePrivateHome`
  (10013, NIP-44).
- **Locally configured** — app config, applied across every user: `RoleIndexer`,
  `RoleBroadcast`, `RoleProxy`, `RoleLocal`, `RoleTrusted`.

A user's editable, role-tagged config is `uc.Relays()` (a `*SessionRelays`):

```go
uc.Relays().Set("wss://relay.example.com", core.RoleOutbox|core.RoleInbox)
uc.Relays().Add("wss://dm.example.com", core.RoleDMInbox)
outboxes := uc.Relays().ByRole(core.RoleOutbox)
all := uc.Relays().All() // map[string]core.Role snapshot
```

`RoleForListKind(kind)` maps a relay-list event kind to its role(s), e.g.
`10002 → RoleOutbox|RoleInbox`.

---

## Streaming fetches

`StreamEvents` is the general, multi-relay streaming primitive: it subscribes to
a filter across many relays and delivers events on a channel as each relay
answers, de-duplicated by id.

```go
feed := client.StreamEvents(ctx,
    nostr.Filter{Kinds: []int{1}},
    relays,
    core.WithLive(), core.WithLimit(50))
for ev := range feed {
    // render ev as it arrives (lazy hydration)
}
```

- Closes on **all-EOSE** (a bounded fetch), `WithLimit(n)`, `ctx` cancellation,
  or `WithTimeout(d)` (default 10s) — whichever is first.
- `WithLive()` keeps it open past EOSE for a live feed.
- `QueryEvents(...)` is the blocking collect-all convenience.
- `uc.StreamNotes(ctx, author)` / `uc.FetchNotes(ctx, author)` are the
  outbox-routed note feeds built on top.

Point it at one relay (e.g. grain's own) for a single-source stream, or at a
user's outbox set for an outbox feed.

---

## Pluggable seam: the Signer

A `Signer` produces signatures for one pubkey. Supply one to publish; omit it
for a read-only context.

```go
type Signer interface {
    PublicKey() string
    SignEvent(event *nostr.Event) error
}
```

The built-in `EventSigner` (a local secp256k1 key via `NewEventSigner(hex)` or
`NewEventSignerFromRandom()`) satisfies it. A consumer can plug in their own —
NIP-46 remote signer, hardware, HSM — by implementing the two methods. (A
`RelayListStore` persistence seam and a pluggable `Logger` are planned; today the
directory caches in memory.)

---

## API reference (essentials)

### `Client`
- `NewClient(*Config) *Client` — owns the shared connection pool.
- `NewUserContext(pubkey string, ...UserOption) *UserContext`.
- `Route(op RouteOp, target string) []string`, `RouteFetch`, `RouteMetadata`,
  `RoutePublish(event)`, `ResolveRelays(pubkey) *UserRelays`.
- `StreamEvents(ctx, filter, relays, ...StreamOption) <-chan *nostr.Event`,
  `QueryEvents(...) []*nostr.Event`.
- `GetUserProfile(pubkey, relayHints) (*nostr.Event, error)`,
  `PublishEvent(event, relays) ([]BroadcastResult, error)`.

### `UserContext`
- `PublicKey()`, `Client()`, `Signer()`, `Relays() *SessionRelays`.
- `Sign(event)`, `Publish(event)`, `SignAndPublish(event)`.
- `FetchNotes(ctx, author, ...)`, `StreamNotes(ctx, author, ...)`,
  `Reply(parent, content)`.
- `PinFixedRelays(read, write)`, `ClearFixedRelays()`, `FixedRelaysEnabled()`.

### `Role`
A `uint16` bitmask; `Has(x)`, `String()`, and the `Role*` constants above.
`RoleForListKind(kind) (Role, bool)`.

### `SessionRelays`
`Set`, `Add`, `Remove`, `Get`, `ByRole`, `All` — concurrency-safe.

### `UserRelays`
Per-target resolution: `Outbox`, `Inbox`, `DMInbox []string`; `ForRole(role)`.

### `BroadcastResult`
`RelayURL`, `Success`, `Accepted`, `Reason`, `Error`, `Message`, `Duration` —
the per-relay outcome of a publish (NIP-20 `OK`).

---

## Runnable examples

Compile-checked examples of the public surface live in
[`client/core/example_test.go`](../client/core/example_test.go) (an external
`core_test` package, so they import the library exactly as you would). They run
in CI via `go test ./...`, so the public API can't drift without breaking the
build.

See also the [outbox relay-pool design doc](design/outbox-relay-pool.md) for the
full architecture and rationale.
