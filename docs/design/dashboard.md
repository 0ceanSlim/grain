# Dashboard rework — design spec

**Status:** draft / spec · near-term UX effort (milestone TBD — candidate for a 0.9.x point release; not post-1.0)
**North star:** *the home page is a **relay operator's dashboard first**, and a **portal into the reference client** second.* It should tell the operator at a glance that the relay is healthy and what it's doing, and make it obvious you can step through the door and browse (search → event → profile) with the client grain ships.

---

## 1. Why

Today's [home.html](../../www/views/home.html) grew by accretion: a **Live Relay Stream** bolted on top, then **Relay Overview** (NIP-11), **Whitelist/Blacklist**, a big **Policy & Limits** block, and **Event Purge** — one long single-column scroll of mostly *config read-outs*. Two problems:

1. **It's config, not a dashboard.** [dashboard.js](../../www/static/js/dashboard.js) fetches `…/relay/config/*` and key lists and renders them — but there are **no live metrics**: no storage fill, no connection count, no events/sec, no kind breakdown. A relay dashboard should lead with vitals.
2. **The live feed sits alone at the top** with nothing beside it, and the whole thing reads as an admin config page rather than a control room + browse portal. (Maintainer: "not sure I like the live feed at the top… the whole dash needs a rework.")

Meanwhile `/admin` (shipped v0.7) is already the **live config editor**. So the home dashboard shouldn't be a second editor — it should be **overview + portal**, and defer deep editing to `/admin`.

## 2. Home vs /admin — the boundary

| | **Home dashboard** (`/`) | **Admin** (`/admin`) |
|---|---|---|
| Role | At-a-glance overview + browse portal | Full live config editing |
| Policy/limits | **Read-only summary** (the effective policy, compact) | The editable forms |
| Whitelist/blacklist | Counts + a peek, "manage →" links to admin | Full add/remove |
| Metrics | **Yes — the headline** | Not its job |
| Browsing | **Yes — search, feed, profile, settings entry points** | No |

This removes the duplication (home currently re-renders config that `/admin` also owns) and gives home a clear identity.

## 3. Proposed layout

A responsive grid, vitals-first, feed no longer alone:

```
┌───────────────────────────────────────────────────────────────┐
│  Relay identity: name · icon · ● Online · software/version      │  ← from NIP-11
├───────────────────────────────────────────────────────────────┤
│  VITALS strip (live, polled):                                   │
│  Storage 42% · Events 1.2M · Connections 37 · Events/min 210 · │
│  Uptime 6d · Pool 112 browsable / 5 connected                  │
├──────────────────────────────────┬────────────────────────────┤
│  📡 Live Relay Stream            │  Activity / metrics panel   │
│  (events, click → /e/)           │  • kinds breakdown (live)   │
│                                  │  • events/min sparkline      │
│                                  │  • top kinds / recent authors│
├──────────────────────────────────┴────────────────────────────┤
│  🚪 Browse the relay (portal): search box · your profile ·      │
│     recent authors · open settings                             │
├───────────────────────────────────────────────────────────────┤
│  ⚙️ Policy at a glance (read-only summary) · "Manage in admin →"│
│     mode (public/private) · auth · rate/size limits · retention │
│     · whitelist N · blacklist N                                 │
└───────────────────────────────────────────────────────────────┘
```

- **Vitals strip** — the new headline. Live, polled on the existing ~5s cadence.
- **Live feed beside a metrics/activity panel** — answers "metrics alongside the live feed." The feed stops being a lonely banner; the two read as "what's flowing" + "what it adds up to."
- **Portal band** — the explicit "this is also a client" entry: a search box (pubkey / npub / note / nevent / NIP-50), a link to your profile (`/p/`), settings, recent authors. Makes the reference-client door obvious.
- **Policy at a glance** — the current Policy/Whitelist/Blacklist/Purge blocks **compressed into a read-only summary** with "Manage in admin →" links, instead of the long editable-looking scroll.

## 4. Metrics — what we can surface

The point of a dashboard. What exists vs. what needs a small addition:

| Metric | Source | Status |
|---|---|---|
| Storage fill (used / total) | `nostrdb.(*NDB).MapUsage()` (the rc2 gauge) | **exists, not exposed** — add to a stats endpoint |
| Client relay pool (browsable / connected / monitors / discovered) | `/api/v1/client/status` | exists |
| Connections (current WebSocket clients) | relay connection tracker (v0.7.1 audit) | needs a read accessor + endpoint |
| Events stored (total, and by kind) | nostrdb count | needs an accessor/endpoint (NIP-45 COUNT machinery may help) |
| Events/min (ingest rate) | a lightweight in-process counter | new, cheap |
| Uptime / build/version | process start + `main.Version` | trivial |

Proposal: one **`GET /api/v1/relay/stats`** (owner-gated like other admin reads) returning `{ storage:{used,total,pct}, connections, events:{total,by_kind}, ingest_per_min, uptime, version }`, plus expose `MapUsage()` there. This is the concrete, near-term slice of roadmap **[#12 Metrics](https://github.com/0ceanSlim/grain/issues/12)** (currently parked at v1.0) — pulling just enough of it forward to make the dashboard real. Anything heavier (historical charts, retention) stays with #12.

## 5. The portal (reference-client door)

grain ships a real client; the dashboard should show it. The portal band wires the existing pieces:
- **Search** — reuse [search.js](../../www/static/js/search.js) (already handles npub/nprofile/nevent/naddr) + NIP-50 text search.
- **Your profile** — `/p/<you>` (which becomes the rich profile per the [event-rendering spec](event-rendering.md)).
- **Feed → event → profile** — clicking a live-stream row opens `/e/` (once event-query robustness lands) and from there the author's `/p/`.
- **Settings** — the tabbed settings page.

This is where the dashboard stops being admin-only and starts being browsable — the same "browsable, not exhaustive" bar as the rendering spec.

## 6. Build approach

- **Go-over-JS.** The vitals strip and policy summary are server-renderable from config + the new stats endpoint; keep JS to the live poll + the feed. The feed's live subscription is being fixed separately (see the live-feed bug) — this rework consumes the fixed feed, it doesn't re-solve it.
- **Phased:**
  1. `GET /api/v1/relay/stats` (+ expose `MapUsage`) and the **vitals strip**.
  2. **Re-layout**: feed beside the activity/metrics panel; collapse policy/whitelist/blacklist into the read-only summary with "manage in admin" links.
  3. **Portal band** (search + profile + settings entry points).
  4. Activity panel richness (kind breakdown, events/min sparkline).

## 7. Open questions

1. **Feed placement:** keep it prominent (beside vitals/metrics) vs. demote it lower so vitals lead. Leaning "vitals strip on top, feed + metrics as the main two-column below."
2. **How much policy on home:** a one-line effective-policy summary, or a richer read-only card set? Everything editable stays in `/admin` regardless.
3. **Metrics scope for v1:** which of §4 ship first (storage + connections + ingest rate are cheap; per-kind counts + history are more work / overlap #12).
4. **Auth:** the stats endpoint owner-gated (like config reads) or public-ish (a relay advertising liveness)? Probably owner-gated to start.
5. **Does this warrant its own milestone**, or fold into a 0.9.x point release alongside the event-query + live-feed fixes (all three are "make the relay's front page actually work")?
