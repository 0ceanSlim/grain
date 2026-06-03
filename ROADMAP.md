# 🌾 GRAIN Roadmap to 1.0

> **The path from today (`v0.7.0`) to a 1.0 release.** This document is the human-readable map; the [GitHub milestones](https://github.com/0ceanSlim/grain/milestones) are the source of truth for individual issues.

---

## 📍 Where we are

[![Latest release](https://img.shields.io/github/v/release/0ceanSlim/grain?label=released&color=blue)](https://github.com/0ceanSlim/grain/releases/latest)
[![Open issues](https://img.shields.io/github/issues/0ceanSlim/grain?color=green)](https://github.com/0ceanSlim/grain/issues)
[![1.0 milestones](https://img.shields.io/badge/milestones%20to%201.0-3-orange)](https://github.com/0ceanSlim/grain/milestones)
[![License](https://img.shields.io/github/license/0ceanSlim/grain?color=lightgrey)](license)

**v0.7.0 just shipped (2026-05-26).** v0.5 closed out the architectural rebirth (MongoDB → embedded `nostrdb`, single-binary, proactive NIP-42 AUTH, client library beta) over April; v0.5.1–v0.5.4 hardened production bugs surfaced under real load (connection-tracking, REQ backpressure, IP blacklist + per-IP rate limiter); v0.6.0 burned down the missing core NIPs (40, 50, 70, 45); v0.7.0 delivered web-based relay administration — a live `/admin` dashboard over NIP-86/NIP-98, an instant reworked login, a seven-theme restyle, and DM privacy by default. A **v0.7.1** hardening patch (a goroutine/connection memory-leak audit) is queued. **v0.8 — relay-as-actor — is current.**

---

## 🗺️ Timeline

```mermaid
gantt
    title GRAIN release timeline
    dateFormat  YYYY-MM-DD
    axisFormat  %b %Y

    section v0.5.0 ▸ Architectural rebirth
    nostrdb migration, AUTH overhaul, single-binary    :done, v05, 2026-04-01, 2026-04-25

    section v0.5.x ▸ Production hardening
    Connection tracking, IP rate-limit, lockup fixes   :done, v05x, 2026-04-25, 2026-04-30

    section v0.6 ▸ Protocol table-stakes
    Small NIPs (40, 70, 45, 50)                        :done, v06, 2026-05-02, 2026-05-07

    section v0.7 ▸ Web admin
    Admin dashboard + NIP-86/98 + theming + login + DM privacy :done, v07, 2026-05-07, 2026-05-26

    section v0.8 ▸ Relay-as-actor
    NIP-29 keypair + outbox client library             :active, v08, after v07, 45d

    section v0.9 ▸ WoT permission groups
    Permission groups + tiered rate limits             :v09, after v08, 60d

    section v1.0 ▸ Sync + polish
    NIP-77 Negentropy + final audit                    :crit, v10, after v09, 30d
```

---

## 🎯 Milestones

### ![v0.5.0](https://img.shields.io/badge/v0.5.0-shipped-success) Architectural rebirth

**Theme:** Single-binary relay with embedded storage and proactive auth.

Dropped the external MongoDB dependency, integrated a custom `nostrdb` fork (`grain-delete`) with real-time physical deletion, embedded the dashboard into the binary, and completed the NIP-42 AUTH flow. v0.5.1 → v0.5.4 followed with critical production fixes: connection-counter underflow, upstream relay pool top-up, REQ backpressure, NIP-42 trailing-slash normalization, NIP-65 outbox-relay mute-list fetch, IP blacklist + per-IP rate limiter, Docker volume path, and the addressable-tag round-trip bug.

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/1)

---

### ![v0.6](https://img.shields.io/badge/v0.6-shipped-success) Protocol table-stakes

**Theme:** Burn down the "missing small NIPs" complaints in one go.

All four shipped, plus a handful of hardening fixes uncovered while running v0.5.4 at scale (slow-consumer disconnects, filter scratch buffer for large pubkey arrays, NIP-42 normalization round 2 with `relay_url_match` knob, NIP-01 OK-prefix correctness for duplicate / replaceable rejections, `logging.stdout` for Docker, app-level `PING`/`PONG`).

| # | Issue | Status |
|---|-------|--------|
| [#49](https://github.com/0ceanSlim/grain/issues/49) | NIP-40 Expiration Timestamp | ✅ closed |
| [#52](https://github.com/0ceanSlim/grain/issues/52) | NIP-70 Protected Events | ✅ closed |
| [#53](https://github.com/0ceanSlim/grain/issues/53) | NIP-45 Event Counts (`COUNT`) | ✅ closed |
| [#48](https://github.com/0ceanSlim/grain/issues/48) | NIP-50 Search capability | ✅ closed |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/2)

---

### ![v0.7](https://img.shields.io/badge/v0.7-shipped-success) Web-based relay administration

**Theme:** Operate the relay from a browser — live config, reworked login, restyle — all on a signed admin API.

Shipped 2026-05-26. What was scoped as an "admin API layer" grew into the relay's web-management release: a full owner-gated **`/admin` dashboard** that tunes every config section live — no YAML edits, no manual restart, and watcher-suppressed reloads that don't drop WebSocket connections — built on **NIP-98** signed HTTP auth and the **NIP-86** management API ([#76](https://github.com/0ceanSlim/grain/issues/76)). Alongside it: an instant, signer-persistent **login rework** on the `nostr-mill` web component ([#86](https://github.com/0ceanSlim/grain/issues/86), [#81](https://github.com/0ceanSlim/grain/issues/81)); a seven-theme **design-token restyle** of dashboard, profile, and Swagger UI ([#88](https://github.com/0ceanSlim/grain/issues/88)); **DM privacy by default** (NIP-17 — gift wraps served only to their p-tagged recipient, [#73](https://github.com/0ceanSlim/grain/issues/73)); browser-decrypted **private mute-list sync** to the blacklist ([#60](https://github.com/0ceanSlim/grain/issues/60)); parallelized mute-list refresh ([#63](https://github.com/0ceanSlim/grain/issues/63), [#85](https://github.com/0ceanSlim/grain/issues/85)); multiple backup relays; and first-run owner provisioning via `GRAIN_OWNER_PUBKEY` / `/setup`.

| # | Issue | Status |
|---|-------|--------|
| [#76](https://github.com/0ceanSlim/grain/issues/76) | Relay admin dashboard (live config UI) | ✅ closed |
| [#50](https://github.com/0ceanSlim/grain/issues/50) | NIP-98 HTTP Auth | ✅ closed |
| [#43](https://github.com/0ceanSlim/grain/issues/43) | Relay API Phase 2 (POST/DELETE) | ✅ closed |
| [#51](https://github.com/0ceanSlim/grain/issues/51) | NIP-86 Relay Management API | ✅ closed |
| [#86](https://github.com/0ceanSlim/grain/issues/86) | Login/signer rework (instant + persistent) | ✅ closed |
| [#73](https://github.com/0ceanSlim/grain/issues/73) | NIP-17 DM privacy (kind:1059 recipient-only) | ✅ closed |
| [#60](https://github.com/0ceanSlim/grain/issues/60) | Admin private mute-list sync | ✅ closed |
| [#63](https://github.com/0ceanSlim/grain/issues/63) | Parallelize per-author mute-list refresh | ✅ closed |

**Deferred to v0.8:** geo/region blocking ([#64](https://github.com/0ceanSlim/grain/issues/64)) and the nostrdb author/id prefix-filter compliance fix ([#72](https://github.com/0ceanSlim/grain/issues/72)) — surfaced during v0.7 but not shipped.

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/3)

---

### ![v0.8](https://img.shields.io/badge/v0.8-current-blue) Relay-as-actor

**Theme:** GRAIN becomes a first-class Nostr citizen.

The architectural prerequisite for WoT, and the current milestone. NIP-29 ships with a relay-owned keypair that gives GRAIN its own identity; the client library graduates from beta with full outbox-model routing. Folded in from v0.7: geo/region blocking, the nostrdb prefix-filter fix, and configurable NIP-50 indexed kinds.

| # | Issue | Scope |
|---|-------|-------|
| [#55](https://github.com/0ceanSlim/grain/issues/55) | NIP-29 Relay-based Groups (+ relay keypair) | Identity foundation |
| [#56](https://github.com/0ceanSlim/grain/issues/56) | Client library: outbox-model relay pool | Library GA |
| [#64](https://github.com/0ceanSlim/grain/issues/64) | Geo/region blocking via GeoIP | Deferred from v0.7 |
| [#72](https://github.com/0ceanSlim/grain/issues/72) | nostrdb author/id prefix-filter compliance | Deferred from v0.7 |
| [#71](https://github.com/0ceanSlim/grain/issues/71) | NIP-50 configurable indexed kinds | Moved up from out-of-scope |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/4)

---

### ![v0.9](https://img.shields.io/badge/v0.9-planned-lightgrey) WoT permission groups

**Theme:** The killer feature.

Composable permission groups built from any combination of explicit whitelist, WoT membership, score thresholds, AUTH state, and admin pubkey. Each group gets its own access, retention, and rate-limit policy. Depends entirely on v0.8.

| # | Issue | Scope |
|---|-------|-------|
| [#14](https://github.com/0ceanSlim/grain/issues/14) | WoT / permission groups | Group model + scoring |
| [#57](https://github.com/0ceanSlim/grain/issues/57) | Per-group rate-limit tiers | Built on the group model |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/5)

---

### ![v1.0](https://img.shields.io/badge/v1.0-planned-red) Sync + polish

**Theme:** The last protocol addition, then ship.

NIP-77 Negentropy is the most complex protocol work in the roadmap; it goes here so that if anything must slip, it slips. Final audit, migration docs, NIP-11 cleanup.

| # | Issue | Scope |
|---|-------|-------|
| [#47](https://github.com/0ceanSlim/grain/issues/47) | NIP-77 Negentropy | Set reconciliation / efficient sync |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/6)

---

## 🪧 Out of scope for 1.0

These were considered and intentionally deferred:

- **NIP-26 (Delegated Event Signing)** — the ecosystem has largely abandoned NIP-26; few clients still implement it. Tagged `Low Priority`, not blocking 1.0. ([#54](https://github.com/0ceanSlim/grain/issues/54))
- **Per-kind blacklisting (NIP-51 kind:30007)** — already achievable via existing `rate_limit.kind_limits` set to 0 per kind. No new feature needed.
- **Whitelist words & relays** ([#18](https://github.com/0ceanSlim/grain/issues/18)) — likely collapses into a permission-group predicate once #14 lands; revisit then.
- **nspam classifier integration** ([#59](https://github.com/0ceanSlim/grain/issues/59)) — nice-to-have spam scoring; post-1.0.
- **Metrics dashboard / endpoints** ([#12](https://github.com/0ceanSlim/grain/issues/12)) — `Good First Issue`, no milestone, post-1.0 if not picked up before. Folds in the v0.5.3 memory-pressure metric work (closed #66) and the v0.6 `setResourceLimit.go` warn-spam follow-up.

---

## 🔄 How this doc stays current

- Every issue tagged `1.0 Requirement` is also assigned a milestone (`v0.8` through `v1.0`).
- This file is updated on milestone close: flip the section header status badge to `shipped`, move the next milestone to `current`, summarise what shipped.
- For day-to-day status, prefer the [milestones page](https://github.com/0ceanSlim/grain/milestones) — it auto-counts open vs. closed.
- Disagree with the sequencing? Open an issue or comment on the relevant milestone.

---

<sub>Last revised after shipping v0.7.0 (2026-05-26): closed milestones v0.5.x / v0.6 / v0.7, promoted v0.8 to current, deferred #64 / #72 from v0.7 into v0.8, and moved #71 up from out-of-scope. A v0.7.1 memory-leak audit patch (#92–#95) is queued.</sub>
