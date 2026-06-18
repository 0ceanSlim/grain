# grain client library guide (0.8.0)

grain's `client/core` package is an **importable, outbox-model Nostr client
engine** in Go. A downstream app builds its own client on it — it does not
reimplement relay routing. The engine owns a shared connection pool, resolves
each user's relay lists, and routes every operation to the right relays under the
[outbox model](https://mikedilger.com/gossip-model/): you read a user's notes
from *their* outbox, and a reply you publish reaches the *parent author's* inbox.

```go
import "github.com/0ceanslim/grain/client/core"
```

`client/core` is pure Go (no cgo, no web/HTTP dependencies). grain's own
`client/api`, `client/session`, `client/connection`, and `client/data` packages
are the **reference consumer** of this surface — a worked example of how to build
on it, not part of its import contract.

> **Status (0.8.0).** This is the **feature-complete client-library surface** —
> 0.8.0 is the bulk of the client work, and everything documented here is usable
> today. NIP-44 encryption (v2 + v3) and NIP-42 AUTH both landed this cycle, and
> both the read/fetch **and** publish paths take a `context.Context` for
> caller-set deadlines and cancellation. The pluggable seams — `Signer`,
> `Logger` (`SetLogger`), and `RelayListStore` (custom directory persistence) —
> are all in place. The one still-pending item is **not** new library surface:
> - `PublishDM` (gift-wrapped NIP-17) — the NIP-44 primitives it needs now exist
>   (below), but the seal/gift-wrap assembly isn't wired yet.
>
> See [the design doc](design/outbox-relay-pool.md) §11 for the rationale.

---

## Contents

- [Quick start](#quick-start)
- [The outbox model & routing](#the-outbox-model--routing)
- [The role model](#the-role-model)
- [Streaming fetches](#streaming-fetches)
- [Relay lists & mailboxes](#relay-lists--mailboxes)
- [Media servers (Blossom + NIP-96)](#media-servers-blossom--nip-96)
- [Encryption (NIP-44)](#encryption-nip-44)
- [AUTH (NIP-42)](#auth-nip-42)
- [The known-relays browser](#the-known-relays-browser)
- [The client tag (NIP-89)](#the-client-tag-nip-89)
- [Pluggable seams](#pluggable-seams)
- [The HTTP API (reference consumer)](#the-http-api-reference-consumer)
- [API reference (essentials)](#api-reference-essentials)
- [Runnable examples](#runnable-examples)

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

ctx := context.Background()
parent := /* the event being replied to */
reply, results, err := uc.Reply(ctx, parent, "well said!")
```

`Reply` builds a NIP-10 reply (thread root + parent markers, thread-wide `p`
tags), signs it as the context user, and publishes it under the outbox model —
the author's own outbox **plus** the parent author's inbox — returning the
per-relay [`BroadcastResult`](#broadcastresult)s.

To publish an event you've built yourself: `uc.SignAndPublish(ctx, event)` (sign +
route + broadcast), or `uc.Sign(event)` then `uc.Publish(ctx, event)` separately.

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

## Relay lists & mailboxes

The engine reads, parses, and assembles a user's replaceable relay-list events.
`FetchUserRelayLists` resolves every kind a relay manager shows, concurrently:

```go
lists := client.FetchUserRelayLists(pubkey) // *UserRelayLists
for _, e := range lists.NIP65 {             // 10002 entries
    fmt.Println(e.URL, e.Read, e.Write)     // read=inbox, write=outbox, both=unmarked
}
fmt.Println(lists.DM, lists.Search, lists.Favorites) // 10050, 10007, 10012
```

`UserRelayLists` covers NIP-65 (`NIP65 []RelayListEntry`, with `Read`/`Write`
markers) plus the NIP-51/37 string lists: `DM` (10050), `Blocked` (10006),
`Search` (10007), `Favorites` (10012), `Private` (10013). For lists whose
`.content` is encrypted, the `Encrypted` flags say which, and `EncryptedContent`
carries the **raw, still-encrypted** blob per list — **grain never decrypts**; it
passes the opaque content through so the consumer's signer can decrypt on demand
(see [Encryption](#encryption-nip-44)).

Parse a single 10002 yourself with `core.ParseNIP65Entries(event)`; it dedupes
and merges read/write markers. To **write** a list, assemble an unsigned event
and sign it with the user context:

```go
entries := []core.RelayListEntry{
    {URL: "wss://out.example.com", Write: true},          // outbox only
    {URL: "wss://in.example.com", Read: true},            // inbox only
    {URL: "wss://both.example.com", Read: true, Write: true},
}
unsigned, err := core.AssembleRelayListEvent(existing, 10002, pubkey, entries)
// unsigned has no ID/Sig — the caller signs:
_, results, err := uc.SignAndPublish(context.Background(), unsigned)
```

`AssembleRelayListEvent` rewrites only the relay tags and **preserves the content
and every non-relay tag** — the same conservative "don't drop data" rule the
profile and media editors use. Tag shape is `["r", url[, "read"|"write"]]` for
10002 and `["relay", url]` for 10050/10006/10007/10012. `existing` may be `nil`
for a first list. Resolution is cached; `WarmUserRelayLists`,
`InvalidateUserRelayLists`, and `ResolveUserRelayLists` (cache-first) manage the
cache — invalidate after the user republishes their own list.

---

## Media servers (Blossom + NIP-96)

The engine resolves a user's media-server lists — Blossom (kind 10063,
[BUD-03](https://github.com/hzrd149/blossom/blob/master/buds/03.md)) and legacy
NIP-96 (kind 10096) — with the same TTL-cached, single-flight directory as relay
lists:

```go
ms := client.ResolveMediaServers(pubkey) // *MediaServers (blocking, cached)
if ms.HasAny() {
    fmt.Println("blossom:", ms.Blossom)  // primary first
    fmt.Println("nip96:", ms.NIP96)
}
```

`MediaServers` is `{Blossom, NIP96 []string; FetchedAt; Negative}` — `Negative`
marks "user has published neither list" (cached briefly so a cold miss doesn't
re-hit the network on every call). `HasAny()` is the "open the picker vs. prompt
to set some up" decision.

- `core.AssembleMediaServerEvent(existing, kind, pubkey, servers)` builds the
  unsigned 10063/10096 event to sign and publish (same preserve-other-tags rule).
- `core.SuggestedMediaServers()` is grain's curated quick-add list
  (`MediaServerInfo`: URL, Kind, Cost, Retention, Mirror, Note, CTA);
  `core.LookupMediaServerInfo(url)` returns capability chips for a known server.
- `WarmMediaServers` / `InvalidateMediaServers` manage the cache.

> The **upload** itself is client-side: the browser computes the file's sha256,
> builds and signs a [BUD-01](https://github.com/hzrd149/blossom/blob/master/buds/01.md)
> (Blossom, PUT) or NIP-96 (multipart POST) authorization with the user's signer,
> and uploads directly to the server (see `www/static/js/blossom-upload.js`). The
> Go library's job is to **resolve which servers** to target; the signed upload
> stays with whoever holds the key.

---

## Encryption (NIP-44)

The built-in `EventSigner` exposes NIP-44 conversation encryption, validated
against the official test vectors. **v2** is the deployed standard and the
default; **v3** is the in-progress draft, opt-in, and binds the event `kind` and
a `scope` into the MAC's authenticated data.

```go
// v2 (default) — the interoperable standard
ciphertext, err := signer.NIP44Encrypt(peerPubKey, "hello")
plaintext, err := signer.NIP44Decrypt(peerPubKey, ciphertext)

// v3 (draft, opt-in) — kind + scope bound into the MAC
ct, err := signer.NIP44V3Encrypt(peerPubKey, kind, scope, []byte("hello"))
pt, err := signer.NIP44V3Decrypt(peerPubKey, kind, scope, ct)
```

The conversation key is derived from the ECDH of the signer's key and the peer's
pubkey, so the **same key holder** decrypts. This is the primitive behind the
private NIP-51/37 lists ([Relay lists & mailboxes](#relay-lists--mailboxes)):
grain hands the consumer the raw encrypted blob, and the consumer decrypts it
with the session signer. The gift-wrapped NIP-17 DM flow (`PublishDM`) is built
on these primitives but not yet wired (see the status note above).

---

## AUTH (NIP-42)

The engine tracks per-relay AUTH challenges so a consumer can answer them with
the session signer. A relay that issues a challenge appears in `AuthRequests()`;
the consumer builds and signs a kind-22242 event and forwards it with
`SendAuth`:

```go
for _, req := range client.AuthRequests() { // []AuthState
    if req.Authed {
        continue // already answered this session
    }
    // Build + sign a kind-22242 event echoing the relay URL and challenge:
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
    if err := client.SendAuth(req.Relay, ev); err != nil { // forwarded on the challenged conn
        // log + surface
    }
}
```

`AuthState` is `{Relay, Challenge, Authed, At}`. Once a relay is answered it
stays authed for the **session**; a fresh challenge from the relay clears the
flag so the consumer re-prompts (your signer will pop up automatically). Drop a
relay with `RemoveAuth(url)`. In grain's reference UI this list **is** the
"Trusted" list — relays you've chosen to authenticate to — so AUTH-for-`trusted`
is opt-in per relay rather than a blanket auto-sign.

---

## The known-relays browser

Everything the engine has seen — from resolutions and config — is browsable, with
live status, NIP-11 metadata, and latency:

```go
all := client.KnownRelays()                 // []string, every relay seen
status := client.KnownRelaysWithStatus()    // []KnownRelayStatus: connected/pinned/leased

info := client.FetchRelayInfo(url)          // *RelayInfo (NIP-11; HTTP GET, TTL-cached)
if info != nil && info.Limitation != nil && info.Limitation.AuthRequired {
    // relay requires NIP-42 AUTH
}

latency := client.PingRelay(url)            // ms (TCP dial), -1 on failure
pings := client.PingRelays(urls)            // map[url]ms, concurrent — for "sort: fastest"
```

`FetchRelayInfo` is a plain HTTP GET with `Accept: application/nostr+json` (not a
pool/WebSocket connection), so the browser can show name, software, supported
NIPs, and the auth/payment flags without holding a relay connection open. It
returns `nil` (cached) when a relay advertises no NIP-11, to avoid retry storms.

---

## The client tag (NIP-89)

`ApplyClientTag` stamps (or strips) the `client` tag on an event you're about to
sign:

```go
core.ApplyClientTag(event, enabled, "grain") // enabled=false strips any client tag
```

It always removes any **foreign** `client` tag first (so re-published events
don't leak another app's attribution), then adds `["client", name]` only when
`enabled`. grain's reference UI defaults the name to `grain` and exposes a
user-facing toggle plus admin defaults; the privacy-respecting default keeps the
tag honest without leaking which app a foreign event came through.

---

## Pluggable seams

A `Signer` produces signatures for one pubkey. Supply one to publish; omit it
for a read-only context.

```go
type Signer interface {
    PublicKey() string
    SignEvent(event *nostr.Event) error
}
```

The built-in `EventSigner` (a local secp256k1 key via `NewEventSigner(hex)` or
`NewEventSignerFromRandom()`) satisfies it, and adds the NIP-44 methods above. A
consumer can plug in their own — NIP-46 remote signer, hardware, HSM — by
implementing the two methods.

**Logging.** The library logs through a pluggable `Logger` — the four
`*slog.Logger` methods (`Debug`/`Info`/`Warn`/`Error`). It defaults to grain's
own logging, so nothing changes until you opt out: `core.SetLogger(yourLogger)`
(any `*slog.Logger` works, or implement the four methods) routes all
client-library logging through your stack instead.

**Persistence.** The relay directory caches per-user resolutions in memory by
default. Set `Config.RelayListStore` to a custom `RelayListStore` — a
`pubkey -> *UserRelays` store (`Get` / `Set` / `Delete` / `Range`) — to back it
with a database that survives restarts or is shared across instances; the
directory keeps the TTL and single-flight logic.

---

## The HTTP API (reference consumer)

grain's `client/api` package wraps this library in an HTTP API for the bundled
web client — the worked reference for consuming `client/core`. The endpoint
groups mirror the sections above:

| Group | What it exposes |
|---|---|
| keys | generate / validate / derive / NIP-19 encode-decode |
| session, login, logout | signer login + session lifecycle |
| profile | resolve + publish kind-0 metadata |
| relay-list | the relay-list build/fetch + fixed-relay endpoints |
| known-relays, relay-ping | the browser + latency sort |
| media-servers | resolve + assemble media-server lists |
| auth | the NIP-42 challenge list + answer/remove |
| stream, events | the streaming feed + event publish |
| client-tag | the client-tag default + toggle |

The full request/response contract is the OpenAPI spec, generated at build via
`make generate` and served by the dashboard at `/api/docs`. Treat that spec as
the authoritative HTTP reference; this guide is the library reference beneath it.

---

## API reference (essentials)

### `Client`
- `NewClient(*Config) *Client` — owns the shared connection pool.
- `NewUserContext(pubkey string, ...UserOption) *UserContext`.
- `Route(op RouteOp, target string) []string`, `RouteFetch`, `RouteMetadata`,
  `RoutePublish(event)`, `ResolveRelays(pubkey) *UserRelays`.
- `StreamEvents(ctx, filter, relays, ...StreamOption) <-chan *nostr.Event`,
  `QueryEvents(...) []*nostr.Event`.
- `GetUserProfile(ctx, pubkey, relayHints) (*nostr.Event, error)`,
  `PublishEvent(ctx, event, relays) ([]BroadcastResult, error)`.
- **Relay lists:** `FetchUserRelayLists(pubkey) *UserRelayLists`,
  `ResolveUserRelayLists`, `WarmUserRelayLists`, `InvalidateUserRelayLists`,
  `FetchRelayList(pubkey, kind)`, `OwnListRelays(pubkey)`.
- **Media:** `ResolveMediaServers(pubkey) *MediaServers`, `FetchMediaServerList`,
  `WarmMediaServers`, `InvalidateMediaServers`.
- **Known relays:** `KnownRelays() []string`, `KnownRelaysWithStatus()`,
  `FetchRelayInfo(url) *RelayInfo`, `PingRelay(url) int`, `PingRelays(urls)`.
- **AUTH:** `AuthRequests() []AuthState`, `AuthChallenge(url)`,
  `SendAuth(url, signed)`, `RemoveAuth(url)`.

### `UserContext`
- `PublicKey()`, `Client()`, `Signer()`, `Relays() *SessionRelays`.
- `Sign(event)`, `Publish(ctx, event)`, `SignAndPublish(ctx, event)`.
- `FetchNotes(ctx, author, ...)`, `StreamNotes(ctx, author, ...)`,
  `Reply(ctx, parent, content)`.
- `PinFixedRelays(read, write)`, `ClearFixedRelays()`, `FixedRelaysEnabled()`.

### `EventSigner` (implements `Signer`)
- `NewEventSigner(hex)`, `NewEventSignerFromRandom()`; `PublicKey()`,
  `SignEvent(event)`.
- NIP-44 v2: `NIP44Encrypt(peer, plaintext)`, `NIP44Decrypt(peer, payload)`.
- NIP-44 v3 (draft): `NIP44V3Encrypt(peer, kind, scope, plaintext)`,
  `NIP44V3Decrypt(peer, kind, scope, payload)`.

### `Role`
A `uint16` bitmask; `Has(x)`, `String()`, and the `Role*` constants above.
`RoleForListKind(kind) (Role, bool)`.

### `SessionRelays`
`Set`, `Add`, `Remove`, `Get`, `ByRole`, `All` — concurrency-safe.

### `UserRelays`
Per-target resolution: `Outbox`, `Inbox`, `DMInbox []string`; `ForRole(role)`.

### `UserRelayLists`
`NIP65 []RelayListEntry`; `DM`, `Blocked`, `Search`, `Favorites`,
`Private []string`; `Encrypted EncryptedFlags`, `EncryptedContent` (raw blobs).
Build with `AssembleRelayListEvent`; parse 10002 with `ParseNIP65Entries`.

### `MediaServers`
`Blossom`, `NIP96 []string` (primary first); `HasAny()`. Build with
`AssembleMediaServerEvent`; suggestions via `SuggestedMediaServers()`.

### `RelayInfo`
NIP-11: `Name`, `Description`, `Software`, `Version`, `SupportedNIPs []int`,
`Icon`, `Limitation *RelayLimits` (`AuthRequired`, `PaymentRequired`, …).

### `AuthState`
`Relay`, `Challenge string`; `Authed bool`; `At time.Time`.

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
