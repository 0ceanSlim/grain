# Event rendering — design spec

**Status:** **Post-1.0 plan** — design complete; build one kind at a time off this doc *after* the 1.0 line lands. Captured now so it isn't lost, not scheduled into 0.9/1.0.
**Scope:** how grain turns a Nostr event into a correct, kind-appropriate view — as a **reusable, importable** library, not just a page in the web client.

> **Near-term overlap:** the outbox-backed *event resolution / query* this spec relies on (§3.5, §8) is also what the current event view needs to reliably find an event across relays. That query-robustness slice may be pulled forward as its own pre-1.0 fix (see ROADMAP); the rich rendering on top of it stays post-1.0.

---

## 1. Why

Today the event view ([`www/static/js/event-page.js`](../../www/static/js/event-page.js) → `renderKindSpecificContent`) dumps raw JSON for every kind (`// TODO: kind-specific rendering`). The feed only snippets kind 1. There is no shared "render a note's content" logic — only `linkifyText` (bare URL links) on the profile page.

We want each kind rendered **the way its NIP intends** (a note reads as a note, a repost shows what was reposted, a long-form article renders its Markdown, a zap shows its amount), and we want that logic to be **importable and reusable** — by grain's own web client, by the feed, by other Go programs that embed the client library, and by non-Go consumers over the API.

## 2. Goals / non-goals

**The bar is *browsable*, not feature-complete.** This is grain's *reference* client — it doesn't have to grow into a fully-supportive social app, but it should feel rich enough to click around: from a note to the pictures in it, to its thread of replies, to the author's profile (their posts, articles, badges, ways to be paid) and back. Richness is the point — it's what makes the reference client actually demonstrate the library.

**Goals**
- One canonical, NIP-correct renderer, reusable across output targets (web, feed, terminal, third-party).
- Importable as a Go package in the client library (`github.com/0ceanslim/grain/client/render`), independent of the HTTP layer.
- Exposed over the API for non-Go consumers.
- **Rich single-event rendering** — media (images/video linked in kind 1, kind 20 galleries) is first-class, not an afterthought.
- **Browsable aggregate views** — a real thread (replies, reposts, reactions, zaps) and a real profile (far more than kind 0), built on the client's outbox subscriptions (§3.5).
- Safe by construction: untrusted event content can never inject markup/script.
- Incremental: ship kind-by-kind behind a generic fallback that always works.

**Non-goals (for now)**
- Composing/publishing events (this is a *view* pipeline).
- Decrypting NIP-04/NIP-17/NIP-59 content in the relay viewer (we label it encrypted; decryption is a signer/client concern).
- Growing into a fully-supportive social client — notifications, DMs, feed algorithms, mutes, etc. Threads and profiles are in scope *for browsability*; the long tail of social features is not.

## 3. Architecture — reusable by design

The reuse question ("referenceable / reimportable / api") is answered by **separating interpretation from output** with a structured intermediate model in the middle. Three layers, each importable on its own:

```
        nostr.Event  (untrusted input)
             │
   ┌─────────▼──────────┐   client/render         ← pure, no I/O, fully testable
   │  Interpret (parse) │   Render(ev) RenderedEvent
   └─────────┬──────────┘
             │   RenderedEvent   ← structured, JSON-serializable IR (the reusable artifact)
   ┌─────────▼──────────┐   client/render/html    ← one of N output renderers
   │  Output renderer   │   HTML(re) template.HTML (sanitized)
   └─────────┬──────────┘   (others: text/terminal, and the raw JSON model itself)
             │
   ┌─────────▼──────────┐   client/render/resolve ← optional, outbox-backed, has I/O
   │  Resolve refs      │   fills author names / quoted events into the IR
   └────────────────────┘
```

### 3.1 The intermediate representation (why this is the reusable core)

A rendered event is **not** returned as an HTML string — HTML only serves the web. Instead `Render` returns a `RenderedEvent`: a structured, JSON-serializable model that any consumer can turn into whatever it needs (HTML page, feed card, terminal output, native UI, OG-image text).

```go
type RenderedEvent struct {
    ID        string       `json:"id"`
    Kind      int          `json:"kind"`
    KindLabel string       `json:"kind_label"`   // from client.KindLabels
    Author    Ref          `json:"author"`       // npub ref, resolvable
    CreatedAt int64        `json:"created_at"`
    Summary   string       `json:"summary"`       // one-line, for feed/OG/title
    Blocks    []Block      `json:"blocks"`        // the body, in order
    Refs      []Ref        `json:"refs"`          // every reference found (for batch resolve)
    Meta      map[string]any `json:"meta,omitempty"` // kind-specific structured fields
    Warnings  []string     `json:"warnings,omitempty"` // malformed tags, etc.
}

type Block struct {
    Type BlockType `json:"type"` // paragraph | heading | image | video | audio |
                                 // link | quote | code | list | divider | callout | raw
    // Populated per type: Segments for paragraph/heading, URL+alt+dims for media,
    // Ref for quote/mention, Level for heading, Language for code, Items for list…
    Segments []Segment `json:"segments,omitempty"`
    URL      string    `json:"url,omitempty"`
    Ref      *Ref      `json:"ref,omitempty"`
    // …
}

type Segment struct {
    Type SegType `json:"type"` // text | link | mention | hashtag | emoji | code
    Text string  `json:"text,omitempty"`
    URL  string  `json:"url,omitempty"`
    Ref  *Ref    `json:"ref,omitempty"`   // for mention (nostr:npub/nprofile/note/nevent/naddr)
}

type Ref struct {
    Type   string `json:"type"`   // npub | nprofile | note | nevent | naddr | addr
    Bech32 string `json:"bech32"` // canonical nostr: entity
    Pubkey string `json:"pubkey,omitempty"`
    EventID string `json:"event_id,omitempty"`
    Relays []string `json:"relays,omitempty"` // hints from the entity/tag
    // filled in by the resolver, when run:
    Resolved *ResolvedRef `json:"resolved,omitempty"` // name/picture, or quoted RenderedEvent
}
```

Key properties:
- **Pure & testable.** `Render` does no network I/O. Given an event it deterministically produces the IR — trivial to unit-test against NIP fixtures.
- **Output-agnostic.** HTML is one renderer over the IR; the IR itself is the reusable artifact (and is exactly what the API returns as JSON).
- **Resolution is opt-in and separate.** `Refs` are typed but empty of profile/quote data until a `Resolver` (which *does* have I/O, via the outbox pool) fills them. Consumers decide when/whether to resolve, so the parser stays pure.

### 3.2 Package layout

- `client/render` — `Render(ev) RenderedEvent`, the IR types, the content-grammar parser. No I/O. No HTTP.
- `client/render/html` — `HTML(re RenderedEvent) template.HTML`, sanitized (goldmark for Markdown blocks, bluemonday for the final pass). Additional renderers live beside it.
- `client/render/resolve` — `Resolver` interface + an implementation backed by `client/core` (outbox lookups for author metadata and quoted events).

### 3.3 API surface (non-Go consumers)

- `GET /api/v1/events/{id}/render` → `RenderedEvent` JSON. `?resolve=1` runs the resolver; `?format=html` returns sanitized HTML instead.
- The existing `GET /api/v1/events/query` gains an opt-in `?render=1` that attaches a `rendered` field per event (for feed/list rendering in one round-trip).

### 3.4 How grain's web client consumes it

grain web is htmx + light JS. Two workable paths (decision in §10):
- **(A) Server-rendered HTML.** The event route returns the page with the body already rendered by `client/render/html`. Simplest, safest, most Go-over-JS. Live updates and lazy ref resolution still need a little JS.
- **(B) JS consumes the IR JSON.** `event-page.js` fetches `RenderedEvent` and walks `Blocks` into DOM. The JS renderer is a *dumb* consumer (no parsing/sanitizing — any HTML in the IR is pre-sanitized by Go), so the same model drives the event page, the feed, and live-arriving events uniformly.

Either way the interpretation + sanitization live once, in Go.

### 3.5 Aggregate views — the browsable surfaces

Single-event rendering (§3.1) turns *one* event into a `RenderedEvent`. But "browsable" means composite surfaces that pull *many* events together — and those need the grain client to open **subscriptions against the author's / event's outbox relays** (the outbox model, via `client/core`). They live one layer up in `client/views` (imports `client/core` + `client/render`); each item inside is a reused `RenderedEvent`. These builders are importable, and the API exposes them for non-Go clients.

**Profile view** — `BuildProfileView(ctx, pubkey) ProfileView`, served at **`/p/<pubkey>` — this *is* the profile.** Today `/p/` renders the kind-0 event alone; that becomes the **identity section** of this view, which wraps it with the rest:
- **Identity** — kind 0 (name, picture, nip-05, about). This is where the *current* `/p/` content lives; for your own profile the existing edit / advanced-editor controls stay here, and for an author viewed as relay owner the ban button stays here too.
- **Ways to be paid** — kind 10133 `payto` targets (NIP-A3) + Lightning/nutzap endpoints (§5.6).
- **Badges** — kind 30008 profile badges, resolved to images.
- **Relays** — kind 10002.
- **Their work, in tabs** — recent **notes** (kind 1), **articles** (kind 30023), reposts/reactions, etc.
- Assembled by resolving the author's relay list → subscribing to their **outbox** for those kinds. This is exactly what "the profile needs a separate render than just the kind 0" means: `/p/` is the profile; the kind-0 render is one section of it.
- API: `GET /api/v1/profile/{pubkey}/view`.

**Thread view** — `BuildThreadView(ctx, eventID) ThreadView`. A post in context:
- The **target post** (`RenderedEvent`) and, above it, its ancestors (the reply-to chain).
- Its **reply tree** — kind 1 replies (NIP-10 `e` markers) and kind 1111 comments (NIP-22 scope tags), nested. 1111 is new, so threading is built on it going forward while still honoring NIP-10 for legacy kind-1 replies.
- **Engagement** aggregated per post — repost count (6/16), reactions (7, grouped by emoji, incl. custom), zaps (9735, summed sats) — each expandable to *who*.
- Assembled by subscribing for events that reference the post's id/address across the relevant relays.
- API: `GET /api/v1/events/{id}/thread`.

**"Live" falls out of this.** Because the subscriptions stay open, new replies/reactions/zaps stream into an open thread and new posts into an open profile — the concrete answer to the parked "what does live mean" question (§10): thread → new replies/engagement, profile → new posts, replaceable event (30023) → a newer version supersedes.

**Cost boundary (the reference-client line).** Aggregate views fan out subscriptions, so they need caps: depth-limit the reply tree, page the profile tabs, load engagement lazily (counts first, "who" on expand), and cap how long subscriptions stay open. This is where "browsable, not exhaustive" gets enforced.

## 4. Content grammar (kind-1-style text)

The parser for free-text content (`content` of kinds 1, 1111, channel messages, comments, etc.) produces `Segment`s / media `Block`s from:

- **URLs** → `link` segment; if the URL (or a NIP-92 `imeta` tag) says it's an image/video/audio, promote to a media `Block`.
- **`nostr:` entities** (`npub`, `nprofile`, `note`, `nevent`, `naddr`) → `mention` segment (people) or `quote` block (events/addresses), carrying a `Ref`.
- **Bare hashtags** `#word` cross-checked against `t` tags → `hashtag` segment.
- **NIP-30 custom emoji** — a `:shortcode:` with a matching inline `emoji` tag (`["emoji", shortcode, url]`) → `emoji` segment that renders the image. Shortcodes *without* an inline tag are resolved against the author's emoji sets (kind 10030 list → kind 30030 sets) by the resolver; unresolved, they stay literal `:text:`. This is shared by note content **and** kind-7 reactions, so it lives in the grammar, not the reaction renderer.
- **Line breaks** → paragraph blocks.
- Everything else → `text`. No Markdown in kind 1 (that's long-form only).

Media honors **NIP-92 `imeta`** (dimensions, blurhash, alt) when present.

## 5. Per-kind rendering spec

Grouped by how they render. Kind labels come from [`client.KindLabels`](../../client/nostr_kinds.go).

### 5.1 Text & social
| Kind | NIP | Rendering |
|---|---|---|
| 1 | 01 | Short note. Full content grammar (§4). Reply context from NIP-10 `e`/`p` (root/reply markers) → "replying to …". |
| 1111 | 22 | Comment. Content grammar + what it comments on, from `A`/`E`/`I`/`K` scope tags. The basis for the thread view (§3.5). |
| 1311 | 53 | Live chat message. Content grammar; link to the live event (`a` tag). |
| 9 | C7 | Chat message (relay group chat). Content grammar. |
| 42 | 28 | Channel message. Content grammar + channel link (`e` root). |

### 5.2 References to other events
| Kind | NIP | Rendering |
|---|---|---|
| 6 | 18 | Repost. "🔁 reposted" + embedded target: parse `content` as the JSON event if present, else a `quote` block resolving the `e` tag. |
| 16 | 18 | Generic repost. Same, with the reposted `k` kind shown. |
| 7 | 25 | Reaction. Empty or `+` → ❤️ (the common case — "mostly hearts"), `-` → 👎; a NIP-30 custom emoji rendered from its inline `emoji` tag image; a bare `:shortcode:` with no inline tag resolved via the reactor's emoji sets; else the literal content. Plus "reacted to" the `e`/`a` target. |
| 17 | 25 | Reaction to a website (`r` tag target). |
| 9802 | 84 | Highlight. Quoted source text + attribution + link to the highlighted event/URL. |
| 1984 | 56 | Report. Report type (from `report`/`p`/`e` tag) + target + reason. |
| 5 | 09 | Deletion request. "requested deletion of" + the `e`/`a` targets + reason. |

### 5.3 Long-form & files
| Kind | NIP | Rendering |
|---|---|---|
| 30023 | 23 | Long-form article. **Markdown → sanitized HTML** (goldmark). Title/summary/image/published-at/tags from `title`/`summary`/`image`/`published_at`/`t`. |
| 30024 | 23 | Draft long-form. Same, badged "draft". |
| 1063 | 94 | File metadata. Render the file (`url` + `m` MIME + `x` hash + `dim`/`blurhash`) as media with a details strip. |
| 20 | 68 | Picture-first post. Gallery of `imeta` images + caption. |
| 21 / 22 | 71 | Video / short-form portrait video. `<video>` from `imeta`/`url` + title/summary. |

### 5.4 Profile & identity
| Kind | NIP | Rendering |
|---|---|---|
| 0 | 01 | Profile metadata. As a *single* event (e.g. opened at `/e/`): an identity card (picture, display name, nip-05, about via content grammar, website/lud16). That same card is the **identity section** of **`/p/<pubkey>`**, which *is* the profile — an aggregate view (§3.5) of posts, articles, badges, and payment targets, not this card alone. |
| 3 | 02 | Follow list. Count + follows as resolvable name chips (`p` tags). |
| 10002 | 65 | Relay list. Read/write relays split out. |

### 5.5 Lists (NIP-51 family) — rendered as titled, typed lists
`10000` mute · `10001` pin · `10003` bookmarks · `10004` communities · `10005` public chats · `10006` blocked relays · `10007` search relays · `10012` fav relays · `10013` private (NIP-37, "encrypted — decrypt to view") · `10015` interests · `10030` emoji · `10050` DM relays · `10063` Blossom · `10096` NIP-96 · `30000` follow sets · `30002` relay sets · `30003` bookmark sets · `30030` emoji sets. **Render:** title/description (`title`/`description`/`d`), then entries grouped by tag type (`p`→people, `e`→events, `a`→addresses, `relay`/`r`→relays, `t`→topics), each a resolvable chip. Private entries (encrypted `content`) shown as a locked count.

### 5.6 Payments & targets
| Kind | NIP | Rendering |
|---|---|---|
| 10133 | A3 | **Payment targets** (`payto:`). A person's declared ways to be paid, across *any* rail — one `["payto", <type>, <address>]` tag per target (`bitcoin`, `lightning`, `ethereum`, `nano`, fiat services, …; `type` lowercase). Render each as a labelled button/link using the type-specific scheme (`bitcoin:<addr>`, `ethereum:<addr>`) or the RFC-8905 fallback `payto://<type>/<address>`; validate the address per type. Multiple `payto` tags coexist. |
| 9734 | 57 | Zap request. Amount (from `amount`), sender, recipient, target, comment. |
| 9735 | 57 | Zap receipt. Amount parsed from the `bolt11` invoice, zapper → recipient, the zapped event/profile, comment. |
| 9321 | 61 | Nutzap (Cashu). Amount + unit, the mint it's drawn on, recipient (`p`), redeemed target (`e`), comment. |
| 10019 | 61 | Nutzap info. The Cashu payment target: accepted mints, relays, and P2PK pubkey. |

**"Who gets paid, and how"** draws from several signals — NIP-A3 is the general, rail-agnostic one; the rest are Lightning/ecash-specific:
- **NIP-A3 payment targets (kind 10133).** The canonical answer — resolving a person surfaces their `payto` buttons (on-chain BTC, Lightning, ETH, fiat, …). This is the one to lead with.
- **Zap splits (NIP-57 App. G).** A `zap` tag on *any event* (`["zap", <pubkey>, <relay-hint>, <weight>]`) routes a Lightning zap to multiple recipients by weight → a "pays: A 60% · B 40%" strip on that event, each recipient a resolvable chip.
- **Profile zap endpoint.** Kind-0 `lud16`/`lud06` → the Lightning "⚡ zappable" affordance.
- **Nutzap target.** Kind-10019 mints + P2PK key → "accepts Cashu nutzaps".

Zap *receipts* (9735) are just the record after the fact.

### 5.7 Channels, communities, calendar
| Kind | NIP | Rendering |
|---|---|---|
| 40 / 41 | 28 | Channel create / metadata. Name, about, picture. |
| 34550 / 4550 | 72 | Community definition / post-approval. Name, description, moderators; approval shows the approved event. |
| 31922 / 31923 / 31924 / 31925 | 52 | Calendar event / calendar / RSVP. Title, time range, location, description. |

### 5.7a Badges (NIP-58) — a three-kind system
Badges span three linked kinds, and rendering an award or a profile-badge grid requires **resolving the definition** for the image (an `a` ref, like any other quote):
| Kind | Rendering |
|---|---|
| 30009 Badge definition | The badge itself: image (`image`, `thumb`), name (`d`), description. The thing the others point at. |
| 8 Badge award | Resolve the definition (`a` tag → 30009) → badge image + name, shown as "awarded by X to Y" (`p` recipients). |
| 30008 Profile badges | The badges a user has *accepted* and displays — a grid of resolved badge images, read from ordered `a`(definition) + `e`(award) pairs. |

### 5.8 Encrypted — never decrypt in the viewer
| Kind | NIP | Rendering |
|---|---|---|
| 4 | 04 (deprecated) | "🔒 Encrypted DM — only the participants can read this." Show sender/recipient (`p`), not content. |
| 13 / 1059 | 59 | Seal / Gift wrap. "🔒 Gift-wrapped — sealed for its recipient." Metadata only. |
| 14 / 15 | 17 | Direct message / file message (only ever seen unwrapped by the recipient's client). Label + note it's a private message. |

### 5.9 Generic fallback (any unlisted kind)
`KindLabels` name (or "Kind N") · a summary line if `content` is plain text · the tags as a typed table · collapsible raw JSON. This is what ships first and what every not-yet-specced kind uses — the current inspector, kept.

## 6. Security

Event content is fully untrusted; a relay viewer renders arbitrary strangers' data. Rules:
- Content **never** becomes markup by string concatenation. The IR carries plain text; the HTML renderer escapes every text node and builds elements from known-safe templates.
- Long-form Markdown is rendered by **goldmark** then passed through **bluemonday** (a strict allowlist: no `<script>`, no event handlers, no `javascript:` URLs, `http(s)`/`nostr`/`mailto` schemes only).
- `nostr:`/URL parsing validates scheme + shape before emitting a link.
- The IR is the trust boundary: anything a consumer places as HTML must come from the Go HTML renderer, not from raw event fields.

## 7. Media & privacy

- Media embeds fetch third-party URLs, leaking the viewer's IP and telling the host someone on this relay opened the event.
- Default proposal: **lazy-load** (only when scrolled into view) and honor `imeta` dimensions to avoid layout shift; consider **click-to-load** posters for video. A relay-side **media proxy** is a later option (removes the leak, adds bandwidth cost) — out of scope for v1, noted as a knob.

## 8. Reference resolution (the outbox tie-in)

Mentions (`npub`/`nprofile`), quotes (`note`/`nevent`/`naddr`), reply context, repost/reaction/zap targets — all are `Ref`s that need a second fetch to become names and quoted cards. This is the **parked outbox-fetch flow**: author's relay list → their outbox → the event.

- `client/render/resolve.Resolver` takes the `Refs` off a `RenderedEvent` and fills them: profile metadata for pubkeys, a nested (depth-capped) `RenderedEvent` for quoted events, using `client/core`'s outbox pool.
- It resolves more than people/quotes: **badge definitions** (kind 30009, for award / profile-badge images), **custom-emoji sets** (kind 10030 → 30030, for shortcodes not carried inline), and **payment targets** (a person's kind-10133 `payto` targets (NIP-A3), plus kind-0 `lud16`/`lud06` and kind-10019 nutzap info) so payment buttons / "⚡ zappable" render without a per-view fetch.
- Depth-cap quotes (e.g. 1 level) to avoid unbounded fan-out; dedupe refs; batch by author-relay; cache resolved profiles/definitions/emoji sets across events in a view.
- In the web client: Go leaves stable placeholders (a chip with the bech32), JS resolves lazily via the render/resolve API and swaps in names/cards, and the live subscription upgrades them as events arrive.
- **Depends on** the two open outbox questions (author resolution strategy; what "live update" means for an immutable event) — so resolution is a later phase, but the IR is designed for it from day one.

## 9. Build order (one at a time, off this doc)

1. **Skeleton:** `client/render` package + IR types + `Render` producing the **generic fallback** for every kind; `client/render/html`; wire the event page to it (replaces the raw-JSON stub, keeps JSON collapsible). Nothing regresses; everything gets a floor.
2. **kind 1** with the full content grammar (§4) + media + NIP-10 reply context. The flagship; unlocks the feed reuse.
3. **kind 0** profile card + **kind 3 / 10002** lists.
4. **6 / 16 / 7** reposts & reactions — reactions cover the ❤️ / `+` / `-` / inline-`emoji` cases now; set-resolved shortcodes land with the resolution phase. Quote/target refs show as unresolved chips first.
5. **30023 / 30024** long-form (goldmark + bluemonday).
6. **Payments & media:** NIP-A3 payment targets (kind 10133 `payto`), zaps (9735 / 9734), zap-split `zap` tags, nutzaps (9321 / 10019); media/files (1063 / 20 / 21 / 22).
7. **NIP-51 list family** (§5.5).
8. **resolution phase:** `client/render/resolve` + lazy JS upgrade (with the outbox questions answered) — this is what turns mention/quote/badge/emoji/payment refs from chips into names, cards, images, and ⚡ affordances.
9. **Badges** (30009 → 8 → 30008) and **custom-emoji set resolution** (10030 → 30030); then the long tail: 5, 1984, 9802, 28 / 72 / 52, encrypted labels.
10. **Profile aggregate view** (§3.5) — `client/views.BuildProfileView`: identity + payment targets + badges + tabs of notes/articles, via outbox subscriptions. **Upgrades `/p/` in place** — today's kind-0-only page and its edit / advanced / ban controls become the identity section of the aggregate.
11. **Thread aggregate view** (§3.5) — `client/views.BuildThreadView`: reply tree (1111 + NIP-10) with reposts/reactions/zaps aggregated per post, live over open subscriptions.

Steps 10–11 are the browsability payoff; once kind 1 (step 2) and resolution (step 8) exist, they can be pulled forward ahead of the long tail.

Each step: a pure `Render` unit test against NIP fixtures + the HTML renderer, then the web wiring.

## 10. Open questions

1. **Web consumption (§3.4):** server-rendered HTML (A) vs JS-consumes-IR (B)? Leaning A for the page body (safest, Go-over-JS) with B-style JS only for live/resolve — but B gives one uniform model for page + feed + live. Pick one.
2. **Live updates:** §3.5 proposes the answer — thread = new replies/engagement, profile = new posts, replaceable (30023) = newer version supersedes. Confirm that's the intended scope, and how long open subscriptions should linger (freshness vs cost).
3. **Media default:** embed-on-load vs click-to-load vs relay media-proxy — and is any of it operator-configurable?
4. **Package home:** `client/render` (sibling to `core`) vs `client/core/render`, and `client/views` for the aggregate builders. Siblings keep `core` lean and each piece independently importable — recommended.
5. **Aggregate-view scope (§3.5):** which tabs the profile carries (notes / articles / reactions / …), how deep the thread tree goes, and how much engagement loads eagerly vs on demand — where exactly the "browsable, not exhaustive" line sits.
6. Eventually fold the consumer API into [`docs/client-library-guide.md`](../client-library-guide.md), like the outbox/discovery docs.
