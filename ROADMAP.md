# 🌾 GRAIN Roadmap to 1.0

> **The path from today (`v0.4.12`) to a 1.0 release.** This document is the human-readable map; the [GitHub milestones](https://github.com/0ceanSlim/grain/milestones) are the source of truth for individual issues.

---

## 📍 Where we are

[![Latest release](https://img.shields.io/github/v/release/0ceanSlim/grain?label=released&color=blue)](https://github.com/0ceanSlim/grain/releases/latest)
[![Open issues](https://img.shields.io/github/issues/0ceanSlim/grain?color=green)](https://github.com/0ceanSlim/grain/issues)
[![1.0 milestones](https://img.shields.io/badge/milestones%20to%201.0-5-orange)](https://github.com/0ceanSlim/grain/milestones)
[![License](https://img.shields.io/github/license/0ceanSlim/grain?color=lightgrey)](license)

**v0.5.0** is the next release — a major architectural rebirth (MongoDB → embedded `nostrdb`, single-binary, proactive NIP-42 AUTH, client library beta). After 0.5 ships, the path to 1.0 is sequenced into five themed minor releases plus the 1.0 finale.

---

## 🗺️ Timeline

```mermaid
gantt
    title GRAIN release timeline
    dateFormat  YYYY-MM-DD
    axisFormat  %b %Y

    section v0.5.0 ▸ Architectural rebirth
    nostrdb migration, AUTH overhaul, single-binary    :done, v05, 2026-04-01, 2026-04-22

    section v0.6 ▸ Protocol table-stakes
    Small NIPs (40, 70, 45, 50)                        :v06, after v05, 30d

    section v0.7 ▸ Admin API layer
    NIP-98 + Relay API Phase 2 + NIP-86               :v07, after v06, 30d

    section v0.8 ▸ Relay-as-actor
    NIP-29 keypair + outbox client library            :v08, after v07, 45d

    section v0.9 ▸ WoT permission groups
    Permission groups + tiered rate limits            :v09, after v08, 60d

    section v1.0 ▸ Sync + polish
    NIP-77 Negentropy + final audit                    :crit, v10, after v09, 30d
```

---

## 🎯 Milestones

### ![v0.5.0](https://img.shields.io/badge/v0.5.0-current-blue) Architectural rebirth

**Theme:** Single-binary relay with embedded storage and proactive auth.

The big swap: drop the external MongoDB dependency, integrate a custom `nostrdb` fork (`grain-delete`) with real-time physical deletion, embed the dashboard into the binary, and complete the NIP-42 AUTH flow.

| # | Issue | Status |
|---|-------|--------|
| [#58](https://github.com/0ceanSlim/grain/issues/58) | Blacklist by mutelist: not working as intended | ✅ closed |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/1)

---

### ![v0.6](https://img.shields.io/badge/v0.6-planned-lightgrey) Protocol table-stakes

**Theme:** Burn down the "missing small NIPs" complaints in one go.

All four are isolated validators or query verbs — no architectural work, just protocol-compliance points clients expect.

| # | Issue | Scope |
|---|-------|-------|
| [#49](https://github.com/0ceanSlim/grain/issues/49) | NIP-40 Expiration Timestamp | Tag-validation gate |
| [#52](https://github.com/0ceanSlim/grain/issues/52) | NIP-70 Protected Events | Auth-gate on `-` tag |
| [#53](https://github.com/0ceanSlim/grain/issues/53) | NIP-45 Event Counts (`COUNT`) | New verb on the relay |
| [#48](https://github.com/0ceanSlim/grain/issues/48) | NIP-50 Search capability | Query surface + nostrdb fulltext |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/2)

---

### ![v0.7](https://img.shields.io/badge/v0.7-planned-lightgrey) Admin API layer

**Theme:** Operators get a remote-management story.

A self-contained dependency chain. Independent of everything else — slot it here because by now the relay has enough surface area to benefit from remote management, and admins feel the pain today.

| # | Issue | Scope |
|---|-------|-------|
| [#50](https://github.com/0ceanSlim/grain/issues/50) | NIP-98 HTTP Auth | Foundation for the next two |
| [#43](https://github.com/0ceanSlim/grain/issues/43) | Relay API Phase 2 (POST/DELETE) | Uses NIP-98 |
| [#51](https://github.com/0ceanSlim/grain/issues/51) | NIP-86 Relay Management API | Uses NIP-98 |
| [#60](https://github.com/0ceanSlim/grain/issues/60) | Admin private mute list sync | Uses NIP-98; finishes #58 |

📂 [View milestone →](https://github.com/0ceanSlim/grain/milestone/3)

---

### ![v0.8](https://img.shields.io/badge/v0.8-planned-lightgrey) Relay-as-actor

**Theme:** GRAIN becomes a first-class Nostr citizen.

The architectural prerequisite for WoT. NIP-29 ships with a relay-owned keypair that gives GRAIN its own identity; the client library graduates from beta with full outbox-model routing.

| # | Issue | Scope |
|---|-------|-------|
| [#55](https://github.com/0ceanSlim/grain/issues/55) | NIP-29 Relay-based Groups (+ relay keypair) | Identity foundation |
| [#56](https://github.com/0ceanSlim/grain/issues/56) | Client library: outbox-model relay pool | Library GA |

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
- **Metrics** ([#12](https://github.com/0ceanSlim/grain/issues/12)) — `Good First Issue`, no milestone, post-1.0 if not picked up before.

---

## 🔄 How this doc stays current

- Every issue tagged `1.0 Requirement` is also assigned a milestone (`v0.6` through `v1.0`).
- This file is updated on milestone close: cross out the section header status badge, move the next milestone to `current`, summarise what shipped.
- For day-to-day status, prefer the [milestones page](https://github.com/0ceanSlim/grain/milestones) — it auto-counts open vs. closed.
- Disagree with the sequencing? Open an issue or comment on the relevant milestone.

---

<sub>Last revised after closing #58 (v0.5.0) and filing #60 (v0.7).</sub>
