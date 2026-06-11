# Design: Outbox-model relay pool + streaming fetch path

> **Status:** Draft for review — no code yet.
> **Issues:** [#56 — Client library: outbox-model relay pool with role-based routing](https://github.com/0ceanSlim/grain/issues/56) (the engine), [#77 — Client fetch path: streaming + concurrent multi-relay queries with lazy UI hydration](https://github.com/0ceanSlim/grain/issues/77) (the consumer).
> **Milestone:** v0.8 (Relay-as-actor) · both tagged `1.0 Requirement`.
> **Scope decision:** full role model (all roles from #56), built in phases.

---

## 1. Why

The built-in Go client (`client/core`) is the library grain ships to downstream apps. Today its pool is a flat, single-user, destructive design that cannot do outbox routing. This doc specifies the reshape and the streaming consumption layer that sits on top of it.

### What the code actually does today (grounding)

| Concern | Current behaviour | Where |
|---|---|---|
| Pool shape | flat `map[string]*RelayConnection`, no roles | [`client/core/relays.go:26`](../../client/core/relays.go) |
| Process model | one global `coreClient` bound to ~5 index relays | [`client/connection/manager.go:10`](../../client/connection/manager.go) |
| Switching to a user's relays | **tears down the whole pool and reconnects** | [`ReplaceRelayConnections`](../../client/core/client.go) `client.go:505` |
| Routing | every query → all connected relays (= index relays) | [`Subscribe`](../../client/core/client.go) `client.go:132` |
| Mailbox use | NIP-65 fetched + cached, **but the profile fetch in the same call still uses index relays** | [`FetchAndCacheUserDataWithCoreClient`](../../client/data/fetch.go) `fetch.go:54` |
| Per-user relay store | JSON strings with only `read`/`write` bools | [`client/cache/relays.go`](../../client/cache/relays.go) |
| Connection cap | `MaxConnections: 10` | [`client/core/config.go:41`](../../client/core/config.go) |

### Two things #77 already got (don't re-do them)

- **Login is already decoupled from the fetch.** `LoginHandler` returns the session immediately and `session.CreateUserSession` kicks a deduped background discovery — [`login.go:103`](../../client/api/login.go), [`FetchUserDataDeduped`](../../client/data/fetch.go) `fetch.go:25`. The issue calls this "80% of the perceived improvement." It's done.
- **Single-flight dedup** for concurrent fetches of the same pubkey is done (same file).

### One concrete bug that explains most of the remaining latency

[`GetUserRelays`](../../client/core/client.go) (`client.go:321`) `select`s on `sub.Done`, but EOSE is routed to `sub.EOSE` ([`relays.go:97`](../../client/core/relays.go)) and `Done` is only closed by `Close()` (deferred to function return). **The mailbox fetch therefore never exits early on EOSE — it always burns the full 5 s timeout**, and the profile fetch runs serially after it. `GetUserProfile` (`client.go:262`) does it correctly. Fixing this + parallelizing the two fetches is the bulk of the "login hydration is slow" symptom, independent of the reshape.

---

## 2. Goals / non-goals

**Goals**
- A pool that holds **many users' relays simultaneously** (hundreds of connections), additively — no destructive swaps.
- **Role-aware routing**: each operation goes to the right relays for the *target* user, not the local user's own set.
- Full role taxonomy from #56, with per-user (event-derived) vs per-install (locally-configured) roles cleanly separated.
- Per-user relay-list **hydration with caching + TTL**.
- **Concurrent, first-wins queries** with **per-relay timeout + backoff** (#77 §1, §6).
- Optional **streaming API boundary** (SSE/NDJSON) + **lazy UI hydration** (#77 §2, §3, §7), with the existing blocking endpoints preserved.
- **AUTH policy**: only sign NIP-42 challenges for `trusted` relays.
- Expose the pool + role assignments so downstream callers can read/edit them.
- **Ship as a reusable, importable library.** A third party should `go get` grain's client package and build their own Nostr client on this outbox engine without reimplementing it or pulling in grain's web/HTTP layer — clean instance-based API, no required package globals, no cookie/HTTP coupling, pluggable signer + storage. (See §11.)

**Non-goals (tracked elsewhere)**
- Implementing individual NIPs (NIP-17 DM ciphertext, NIP-50 search semantics) — only their *relay-list* discovery is in scope here.
- Relay-*server* changes — this is purely client-library work.
- NIP-77 negentropy set reconciliation (#47, v1.0).
- Cross-tab subscription multiplexing (#77 "out of scope").

---

## 3. Relay role taxonomy

Roles fall into three classes by where the relay set comes from. (Research §12 pinned down the concrete kinds — several roles the first draft called "local" actually have standard published list events.)

- **Per-target event-derived** — resolved for *any* user you interact with, from their published events; these populate the per-target directory (§4.4).
- **Self-only event-derived** — the *logged-in user's own* published preferences; hydrated into the session config (§4.3) on login (some need the user's signer to decrypt). Not fetched for other users.
- **Local-only** — app/session config; no standard published kind.

| Role | Kind / source | NIP | Class | Notes |
|---|---|---|---|---|
| `Outbox` | 10002 `write` | 65 | per-target | publish a user's notes / fetch their notes |
| `Inbox` | 10002 `read` | 65 | per-target | replies / zaps to a user are delivered here |
| `DMInbox` | **10050** | 17 | per-target | gift-wrapped DM delivery; `["relay", url]` tags, empty content |
| `PrivateHome` | **10013** (NIP-44 encrypted) | 37 | self-only | "relays for private content" (draft wraps); owner-only — needs the user's signer to decrypt |
| `Search` | **10007** | 51 | self-only (+ local) | NIP-50 search relays |
| `Blocked` | **10006** | 51 | self-only (+ local) | pool never dials these |
| `Favorite` | **10012** (+ 30002 sets) | 51 | self-only (+ local) | browsable favorites; surfaced in UI |
| `Indexer` | client config (seed list) | — | local | resolve anyone's NIP-65/17 lists; default 5 in [`config.go`](../../client/core/config.go) |
| `Broadcast` | client config | — | local | own publishes mirror here for wider fan-out |
| `Proxy` | client config | — | local | aggregator override — short-circuits outbox expansion |
| `Local` | client config | — | local | same device / LAN; latency-preferred |
| `Trusted` | client config | — | local | **only** relays we'll sign NIP-42 AUTH for |

`PrivateInbox` from the first draft is **dropped** — no standard "private inbox" kind exists; kind 10013 covers the private-relay need as `PrivateHome`. A single URL can hold several roles (NIP-65 "both" = Outbox+Inbox; an indexer can also be trusted). The per-target directory (§4.4) only ever needs Outbox/Inbox/DMInbox; the self-only roles load into the session config (§4.3) from the logged-in user's own kinds, with local config as override/fallback.

---

## 4. Data model

Four layers. The bottom layer is the *default* lists; above it sits **one shared, growing connection pool**; above that, *per-session* config decides which relays each user uses; and a per-target-user directory feeds routing when you interact with someone else. Separating these is what lets the pool grow to hundreds of shared sockets while each user still routes their own way.

### 4.0 The shape in one paragraph

There is **one default relay list** (app config) so the relay works before anyone logs in and so new sessions have a starting point. As the relay runs and users interact, it dials more and more relays into **one shared connection pool** — the "hundreds of websockets" of the outbox model, ref-counted and shared across sessions. When a user logs in they get a **per-session relay config**: their own event-derived relays (NIP-65 outbox/inbox, NIP-17 DM…) **plus** the default local-role relays, all **editable by that user and persisted in session cache**. Every action selects a read-set or write-set *out of the shared pool*, computed from routing (§5). Interacting with another user pulls *that* user's outbox/inbox into the pool on demand — and **by default the client writes to other users' relays as well as its own** (that is the whole point of outbox). A user *can* explicitly pin to fixed relays ("only write to X / read from Y"), but that disables outbox routing and is opt-in and discouraged (§4.3).

### 4.1 App defaults — config: bootstrap + per-session seed

- The seed `index_relays` + default local-role relays in [`config.go:30`](../../client/core/config.go).
- Used directly for **not-logged-in** relay operations (mutelist fetch, dashboard, telemetry).
- Copied as the **starting point** for each new session's local-role set (§4.3).

### 4.2 Shared connection pool — process-wide, role-agnostic, ref-counted

One live socket per URL, **shared by every session that needs it**, torn down only when nobody does. This is the pool that grows to hundreds of connections.

```go
// RelayConnection gains lease bookkeeping; the socket itself carries no roles.
type RelayConnection struct {
    URL    string
    Conn   *websocket.Conn
    Status ConnectionStatus
    // ... existing fields (writeChan, done, closeOnce, Subscriptions) ...

    leases   int        // active acquire() holders; 0 ⇒ eligible for idle evict
    idleAt   time.Time  // when leases last hit 0
    lastErr  time.Time  // for dial backoff
    failCnt  int        // consecutive dial/read failures ⇒ exponential backoff
}
```

Pool API (replaces the destructive `ReplaceRelayConnections` model):

```go
func (rp *RelayPool) Acquire(url, reason string) (*RelayConnection, error) // dial-on-demand, leases++
func (rp *RelayPool) Release(url string)                                    // leases--, start idle timer at 0
```

- **Connect-on-demand:** `Acquire` dials only if absent and not `Blocked`.
- **Idle eviction:** a sweeper closes connections with `leases == 0 && now-idleAt > IdleTTL`.
- **Bounded dials:** a semaphore caps *concurrent* dials; a soft cap on *total* connections triggers LRU eviction of idle ones (the outbox pool routinely wants hundreds — see §10 config).
- **Backoff:** failed dials set `failCnt`/`lastErr`; `Acquire` skips a relay still inside its backoff window so one dead relay isn't re-hammered.

### 4.3 Per-session relay config — session cache, user-editable

The per-user overlay that makes the client behave exactly how that user wants. On login it is **seeded** from §4.1 defaults (local roles) **plus** the user's resolved event-derived relays, then it is **editable and persisted in session cache**. This extends today's `cache.clientRelays[pubkey]` ([`cache/relays.go`](../../client/cache/relays.go)), which already stores a per-user relay list but only with `read`/`write` bools — enrich it to the full `Role` set below.

It holds, per logged-in pubkey:
- the user's own **per-target-shaped** roles — `Outbox` / `Inbox` / `DMInbox` (also in the directory §4.4)
- the user's own **self-only event-derived** roles, hydrated from their published lists — `Search` (10007) / `Blocked` (10006) / `Favorite` (10012) / `PrivateHome` (10013, decrypt with the user's signer)
- **local** roles inherited from defaults — `Broadcast` / `Indexer` / `Proxy` / `Local` / `Trusted` — now user-overridable
- an optional **fixed-relay override** (default **OFF**): pin the client to specific relays ("only write to X", "only read from Y"). This **turns the outbox model off** — replies stop reaching other users' inboxes — so it is explicit opt-in and **not recommended** unless the user deliberately wants a single-/fixed-relay client.

```go
type SessionRelays struct {
    mu    sync.RWMutex
    roles map[string]Role // url → bitmask, scoped to THIS session
}
```

`Role` is a bitmask shared across the routing code:

```go
type Role uint16
const (
    RoleOutbox Role = 1 << iota
    RoleInbox
    RoleDMInbox
    RolePrivateHome // NIP-37 kind 10013 (encrypted, self-only)
    RoleProxy
    RoleBroadcast
    RoleIndexer
    RoleSearch
    RoleLocal
    RoleTrusted
    RoleFavorite
    RoleBlocked
)
```

When nobody is logged in, routing falls back to §4.1 defaults directly.

### 4.4 Event-derived directory — other users, per target user, TTL-cached

```go
type UserRelays struct {
    Outbox    []string // NIP-65 kind 10002 write
    Inbox     []string // NIP-65 kind 10002 read
    DMInbox   []string // NIP-17 kind 10050
    FetchedAt time.Time
    Negative  bool // cached "no list published" — short TTL
}

type RelayDirectory struct {
    mu  sync.RWMutex
    m   map[string]*UserRelays // pubkey → lists
    ttl, negTTL time.Duration
}
```

This generalizes today's `Mailboxes` (which only has Read/Write/Both). The logged-in user is just the one pubkey whose entry *also* carries the editable, persisted session overlay of §4.3; everyone else is resolve-and-cache only.

---

## 5. Routing table

`Route(op, targetPubkey) []string` composes the session's roles (§4.3) + the target's event-derived roles (§4.4). **By default this is full outbox routing** — a reply writes to **both** the author's outbox *and* the target's inbox (so it lands where the target reads), and reads hit the author's outbox. Routing collapses to a fixed relay set **only** if the user has explicitly enabled the fixed-relay override (§4.3), which turns outbox off. If the session has `Proxy` relays configured, feed-fetch ops short-circuit to proxy. The resolved URLs are then `Acquire`d from the shared pool (§4.2).

| Operation | Relays resolved |
|---|---|
| Publish own note | own `Outbox` ∪ `Broadcast` |
| Reply / react / zap to X | own `Outbox` ∪ X's `Inbox` |
| Fetch X's notes | X's `Outbox` ∪ `Local` (or `Proxy` if set) |
| Resolve X's metadata / relay-lists | X's `Outbox` if known, else `Indexer` |
| DM to X | X's `DMInbox` |
| Own private / draft events | `PrivateHome` |
| Search query | `Search` |
| Background telemetry | `Indexer` only — never the user's relays |

Fallbacks: empty event-derived set ⇒ fall back to `Indexer` (resolve) or `Outbox`→`Indexer` (fetch), so a user with no published list still works.

---

## 6. Hydration

Resolving a target user's event-derived roles:

1. Look up `RelayDirectory` cache. Fresh hit ⇒ return.
2. Miss/stale ⇒ query `Indexer` relays for that pubkey's kind **10002** (NIP-65) and kind **10050** (NIP-17 DM list) concurrently, first-wins (§7).
3. Cache positive result with `ttl`; cache "nothing found" with `negTTL` (short) so a flood of unknown pubkeys can't DoS the indexers.
4. Single-flight per pubkey (reuse the `flights` pattern already in [`fetch.go:17`](../../client/data/fetch.go)).

This is where today's `GetUserRelays` becomes a generic per-target resolver instead of a one-shot "current user" call.

---

## 7. Query semantics (#77 §1, §6)

New helpers alongside `Subscribe`, replacing the serial 5 s pattern:

```go
// First valid event across relays; keeps the sub open briefly to collect a
// couple more for replaceable-event recency, then cancels the rest.
func (c *Client) QueryFirst(ctx, filters, relays, opts) (*nostr.Event, error)

// Streams events as they arrive; caller decides when enough is enough.
func (c *Client) QueryStream(ctx, filters, relays, opts) (<-chan *nostr.Event, error)
```

- **Concurrent fan-out**, return on first valid event; brief grace window for replaceable recency; cancel losers via context.
- **Per-relay timeout:** open cap ~1–2 s, EOSE cap ~4–5 s, each on its own ctx — one dead relay adds at most its own cap (acceptance criterion of #77).
- **Fix the `GetUserRelays` EOSE bug** and route mailbox + profile fetches through `QueryFirst` concurrently.

---

## 8. Streaming boundary + lazy UI (#77 §2, §3, §7)

- API endpoints (`/api/v1/user/profile`, `/user/relays`, `/events/query`) gain an **optional** SSE / chunked-NDJSON mode (e.g. `Accept: text/event-stream`). **The existing blocking JSON shape stays the default** — no breakage for current callers (acceptance criterion).
- `/api/v1/cache` response grows `stale` / `refreshing` flags so the UI knows when to shimmer vs. trust the value (#77 §7).
- UI: server-renders skeletons immediately (name + npub from `parseIdentifier`), small JS hydrates as SSE events land. **Per the project's "Go over JS" rule, keep the hydration layer minimal** — server-rendered placeholders + a thin SSE listener, not a client framework. Reuse `nostr-mill` where it already covers a piece.

---

## 9. AUTH policy (NIP-42)

The client pool does not handle AUTH today. New rule: on an `AUTH` challenge, sign **only if** the relay URL carries `RoleTrusted` in `LocalRoles`; otherwise ignore. Default is refuse.

---

## 10. Config & migration

`MaxConnections: 10` is wrong for an outbox pool. Proposed knobs (these also feed the future dashboard client-config section, #80):

| Knob | Meaning | Default (proposed) |
|---|---|---|
| `max_connections` | soft cap on total live sockets; LRU-evict idle over this | 256 |
| `max_concurrent_dials` | dial semaphore | 16 |
| `idle_ttl` | evict a 0-lease connection after this | 120 s |
| `relay_list_ttl` / `relay_list_neg_ttl` | hydration cache | 1 h / 1 min |
| `open_timeout` / `eose_timeout` | per-relay query caps | 2 s / 5 s |

**Migration / compatibility**
- `ReplaceRelayConnections` / `SwitchToUserRelays` destructive teardown is **deprecated**, not deleted — kept as thin shims (acquire the URLs, no global swap) until callers migrate.
- The single global `coreClient` stays; only its pool internals change — it owns the one shared pool (§4.2), not a per-user set.
- Per-session relay config persists in session cache (enrich `cache.clientRelays[pubkey]` to the full `Role` set); seeded on login from defaults + resolved event-derived relays, editable by the user thereafter.
- Preserve the recent leak fixes (#93/#94): `signalDone`/`closeOnce` semantics carry over unchanged into the leased connection.

---

## 11. Library API surface & reusability

This is the library grain ships to downstream apps, so the outbox engine must **stand alone** — importable as `github.com/0ceanslim/grain/client/core` (and siblings) without dragging in grain's web layer. A consumer builds their own client on it; they do not reimplement outbox routing.

### Principles
- **Instance-based, no hidden globals.** The pool, directory, and routing hang off a `*core.Client` the caller constructs. grain's package-level `coreClient` ([`connection/manager.go:10`](../../client/connection/manager.go)) and global cache are then *grain's own* single instance, not a requirement of the library.
- **No HTTP/cookie coupling in core.** The per-session relay config (§4.3) is a library type (`core.UserContext` below), independent of grain's `client/session` cookie machinery. grain's web layer wraps it; a CLI or another app uses it directly.
- **Pluggable seams** (interfaces, not concrete deps):
  - `Signer` — sign/encrypt for a pubkey (local key, NIP-07, NIP-46). Read-only callers pass none.
  - `RelayListStore` — persist/restore the §4.4 directory + §4.3 session config (in-memory default; grain plugs its cache; others plug a DB).
  - `Logger` — keep the `server/utils/log` abstraction injectable.
- **`context.Context` on every network method** for cancellation/deadlines.
- **Exported role/routing types** so callers can inspect and edit assignments (the #56 "expose pool + roles" requirement): `Role`, `UserRelays`, `SessionRelays`, `Route(op, target)`.

### Sketch of the importable surface
```go
client := core.NewClient(core.DefaultConfig())       // owns the shared pool (§4.2)

// A user context = the per-session relay config (§4.3) + optional signer.
uc := client.NewUserContext(pubkey, core.WithSigner(mySigner))
uc.Relays().Set(url, core.RoleOutbox|core.RoleInbox)  // user edits their own config
uc.Relays().PinFixed(url)                             // opt-out: pin to fixed relays — DISABLES outbox, discouraged

// Routing is automatic; the caller names the intent, not the relays.
notes, err := uc.FetchNotes(ctx, authorPubkey)        // → author's Outbox
err = uc.Reply(ctx, parent, content)                  // → own Outbox ∪ parent author's Inbox
err = uc.PublishDM(ctx, recipient, content)           // → recipient's DMInbox
profile, err := client.ResolveProfile(ctx, anyPubkey) // → Outbox else Indexer
```
grain's `client/api`, `client/data`, `client/session`, and the global `coreClient` become the **reference consumer** of this surface — a worked example of how to build on the library, not part of its import contract.

### Package boundaries
- `client/core` — the engine (pool, directory, routing, `UserContext`, `Signer`/`RelayListStore` interfaces). Importable, no web deps.
- `client/cache`, `client/connection`, `client/data`, `client/session`, `client/api` — grain's application layer / reference consumer built on top.

---

## 12. Research questions — resolved

Settled against the specs (June 2026):

- **DMInbox = NIP-17 kind 10050.** `["relay", url]` tags, empty content, replaceable. Per-target.
- **`PrivateInbox` dropped.** No standard "private inbox" relay list exists. The private-relay need is met by **NIP-37 kind 10013** ("relays for private content" / draft wraps) — **NIP-44 encrypted**, so resolvable only for the logged-in user (needs their signer). Mapped to `PrivateHome` (self-only).
- **Search relays = NIP-51 kind 10007** — a real user-published list. Also found alongside it: **10006 blocked**, **10012 favorite** (+ **30002** relay sets). So Search / Blocked / Favorite are *self-only event-derived* (hydrate the logged-in user's own lists), with local config as override — not local-only as the first draft assumed.
- **Directory persistence: no.** Cold-resolve per-target lists from indexers on demand, protected by the TTL cache (§4.4) + single-flight. Optional persistence can come later via the `RelayListStore` seam (§11) if indexer load justifies it.

Still open (not blocking — tune under load):

- **`MaxConnections` default (256)** wants a real load sanity check.

---

## 13. Phasing (maps to acceptance criteria)

| Phase | Deliverable | Closes |
|---|---|---|
| **0** | Fix `GetUserRelays` EOSE bug; parallelize mailbox+profile fetch | #77 latency symptom — shippable alone |
| **A** | Connection layer reshape: leases, connect-on-demand, idle evict, bounded dials, backoff | #56 lifecycle |
| **B** | Role tables (4.2/4.3) + hydration (§6) + routing (§5) | #56 routing, proxy override |
| **C** | `QueryFirst`/`QueryStream` + per-relay timeout/backoff | #77 §1, §6 |
| **D** | SSE/NDJSON boundary + `cache` warmth flags + lazy UI skeletons | #77 §2, §3, §7 |
| **E** | AUTH policy for `trusted`; finalize the importable API (§11) — exported role/routing types, `UserContext`, `Signer`/`RelayListStore` seams, instance-based (no required globals) | #56 AUTH + exposure |
| **F** | **Full rewrite of `docs/client-library-guide.md` for 0.8.0** — outbox quick-start, routing walkthrough, API reference, and a runnable example of a *third-party* client built on the library | library GA docs |

Phases 0–C and E are Go-only and **natively testable** (no cgo — this is client-side, unlike nostrdb). Phase D adds the only JS surface; keep it thin. Phase F is docs. Unit-test routing resolution, lease lifecycle, and role math against a mock relay — and the doc's third-party example should compile in CI so the public API can't silently drift.

---

## 14. Acceptance recap

- **#56:** multi-role pool · per-user hydration w/ TTL · role-aware routing · proxy override · connect-on-demand + idle evict + bounded dials (hundreds of conns) · AUTH only for trusted · pool/roles exposed to callers.
- **#77:** login→dropdown < 500 ms warm (already mostly via background fetch) · profile first paint name+npub < 500 ms cold · one dead relay ≤ its own per-relay timeout · existing blocking endpoints unchanged.
