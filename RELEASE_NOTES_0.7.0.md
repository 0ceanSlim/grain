# GRAIN🌾 v0.7.0

**Web-based relay administration**
Released: _TBD_
**Full Changelog**: https://github.com/0ceanSlim/grain/compare/v0.6.0...v0.7.0

---

v0.6 finished the core protocol work. v0.7 focuses on operating the relay: a web dashboard for live configuration, a reworked login flow, and a restyled interface.

You no longer have to hand-edit YAML and restart to change how the relay behaves. Sign in with your Nostr key, open `/admin`, and every setting — rate limits, whitelists, blacklists, purging, backups, relay identity — is a form you can edit and apply from the browser. Login is faster, the frontend has a proper theme system (seven themes, switchable from the header), and your private mute list can feed the relay's blacklist without exposing your key.

This is built on two new standards — **NIP-98** (signed HTTP auth) and **NIP-86** (the relay management API) — so every dashboard action is a signed, owner-only operation. Operators who prefer the API get a full programmable control surface; the dashboard is just a client for it.

---

## 🖥️ Relay admin dashboard

A new owner-gated dashboard at **`/admin`** configures the relay live — no YAML editing, manual restart, or shell access required.

- Sign in with your Nostr key and the dashboard opens to your relay's current configuration.
- Each setting is a real form with plain-language explanations of what it does and when it takes effect. Edit a section, hit **Save**, and a confirm modal shows a live progress spinner while the relay reloads.
- A floating Save/Discard bar appears only when you've changed something, with an "unsaved" marker so you always know where you stand.

**Every section is editable from the UI:**

| Section | What you can tune |
|---|---|
| **Logging** | Log level/format, and a checkbox grid to quiet noisy components |
| **Auth** | NIP-42 AUTH on/off, relay URL, strict vs. host URL matching |
| **Event purge** | Retention intervals, which kinds to purge, whitelist protection |
| **Time constraints** | How old / how far in the future an event can be (3-mode picker) |
| **Rate limits** | Global + per-category + per-kind rates and event-size caps |
| **Resource limits** | CPU, memory ceiling, heap target |
| **Server** | Listen address, timeouts, connection + subscription caps |
| **Backup relays** | **Now multiple URLs** — run a public blaster + a private archive at once |
| **Whitelist** | Pubkey / kind / domain lists with profile previews and unified hex/npub entry |
| **Blacklist** | Banned pubkeys/words, live IP block/unblock, mute-list authors |
| **Ops** | Live stats, relay identity (name, icon, banner, policy URLs), cache refresh |

**First-run setup is one click.** A fresh deployment no longer needs a hand-edited metadata file to reach the dashboard. Set `GRAIN_OWNER_PUBKEY` (hex or npub) at startup, or just visit `/setup`, sign in, and claim ownership. An "unclaimed" banner shows until you do.

<details>
<summary><b>For operators & developers — how the dashboard writes config safely</b></summary>

- Every dashboard action is a **NIP-86 JSON-RPC call**, authenticated with **NIP-98** and gated to the relay owner.
- Saves stage a config section (`grain_update*`, returning `restart_pending:true`) and apply via `grain_reloadconfig`.
- **Watcher suppression**: admin writes register the file path in a TTL'd set before saving so the fsnotify watcher skips the change instead of firing the 3-second restart loop — *editing config in the UI doesn't drop every WebSocket connection.*
- All writes go through a package-wide mutex and `AtomicWriteFile` (tmp+rename, with an in-place fallback for bind-mounted Docker / k8s ConfigMap / systemd PrivateTmp environments where rename returns EBUSY).
- The Ops section's identity editor fires only the changed `changerelay*` methods, so you sign once per edited field.
- Tracked under [#76](https://github.com/0ceanSlim/grain/issues/76).
</details>

---

## 🎨 Interface restyle and theming

The web UI has been rebuilt on a design-token system.

- **Seven themes**, switchable live from the header: the original dark/light plus `midnight`, `matrix`, `grain` (the brand identity), `solar` (Solarized Dark), and `candy` (pastel light). Your choice is remembered.
- A restyled header with centered search, a single login button that becomes your avatar + name, and a theme switcher. Trimmed footer. The dashboard, profile, and settings pages all moved onto the new look.
- **Self-hosted fonts** (Space Grotesk, JetBrains Mono, Comfortaa) — no CDN dependency.
- The API docs at **`/api/docs`** got the same treatment: a token-aware Swagger UI, now linked from the nav dropdown and footer with a live example ([#88](https://github.com/0ceanSlim/grain/issues/88)).

<details>
<summary><b>For developers — the token system</b></summary>

A full design-token namespace in `input.css` (surface / text / border / accent + secondary / semantic / typography / radius / shadow), a `window.GrainTheme` swapper backed by `localStorage`, and interaction-state tokens (link, selection, hover/active surfaces, scrollbar, focus ring, motion durations) driving element defaults. A new theme is one `:root[data-theme=…]` block plus one entry in `theme.js`. A `/style-test` page renders every token and widget for previewing.
</details>

---

## 🔑 Faster, more reliable login

Signing in previously blocked on a relay fetch that could take seconds — sometimes up to a minute. That's been reworked:

- **Sign-in returns immediately.** The session is created as soon as you authenticate; your profile and relay list load in the background and fill in as they arrive.
- **The signer persists across reloads.** Whatever method you used — browser extension, bunker, encrypted key, or Amber — the relay restores your signer on reload instead of re-prompting. If it does need you, a "🔑 Reconnect signer" pill appears rather than opening a modal mid-edit.
- **Amber is supported on Android** for the same-device mobile flow.
- **No private keys on the server.** The session stores only your public key and signing method; key material stays in your browser.

The login UI itself is now built on **`nostr-mill`**, a Web Component that handles every signing method through one consistent interface and retints to match your active theme.

<details>
<summary><b>For developers — what changed</b></summary>

- `CreateUserSession` creates the session + cookie immediately and runs the user-data fetch in a goroutine — login returns in ~2ms regardless of relay latency. The fetch is single-flighted so polling clients don't stampede outbox relays.
- `/api/v1/cache` returns `{publicKey, npub, pending:true}` immediately on a miss and kicks a deduplicated background fetch — no blocking, no 500 for users with no published kind-0.
- A generic `restoreSigner` replaces the old NIP-07-only reconnect (`MILL.restore({method, pubkey})`); logout calls `MILL.clearRestoreState`.
- The bespoke 438-line login modal and five per-method auth handlers were deleted in favor of the `<nostr-signer>` element (bundled mill **1.5.0**, no runtime CDN). The plaintext private-key field was removed from the session struct.
- Tracked under [#86](https://github.com/0ceanSlim/grain/issues/86) and [#81](https://github.com/0ceanSlim/grain/issues/81).
</details>

---

## 🛡️ DM privacy by default

grain now protects direct-message metadata. Gift-wrapped DMs (kind 1059) are served **only to the recipient they're addressed to** — previously any client could request all of them and reconstruct who was messaging whom and when, even without decrypting the contents.

This is on by default with no opt-out, applied to live subscriptions, historical queries, and counts alike.

<details>
<summary><b>For developers — NIP-17 enforcement</b></summary>

Kind-1059 is served only to a connection AUTHed as a pubkey p-tagged on the event. Unauthed `REQ {"kinds":[1059]}` → `auth-required`; the result set is post-filtered so protected events never reach a non-recipient regardless of filter shape. COUNT is constrained to the caller's own `#p`. The broadcast gate is resolved once per event so the non-DM hot path skips the per-client auth lookup. Tracked under [#73](https://github.com/0ceanSlim/grain/issues/73).
</details>

---

## 🧹 Blacklist improvements

- **Private mute-list sync.** Most of a NIP-51 mute list lives in encrypted content the relay can't read. The dashboard decrypts your own mutes *in your browser* and sends the pubkeys to the relay's blacklist — the relay never sees your key. Public stats show only a count, never the muted pubkeys ([#60](https://github.com/0ceanSlim/grain/issues/60)).
- **Mute-list authors now persist on reload** (they previously rendered empty), and the per-author refresh runs in parallel rather than sequentially (≈4× faster) ([#85](https://github.com/0ceanSlim/grain/issues/85), [#63](https://github.com/0ceanSlim/grain/issues/63)).

---

## 🐛 Other fixes & polish

- **Home dashboard stats** showed `undefined` for rate/size limits and a wrong auth status — fixed the field-name mismatches ([#82](https://github.com/0ceanSlim/grain/issues/82)).
- **Profile "view event JSON"** reloaded the page instead of showing the JSON — now toggles in place ([#75](https://github.com/0ceanSlim/grain/issues/75)).
- Profile metadata hydrates more reliably after login, and the admin link no longer goes missing for owners with no published profile.
- The login button shows a "Signing in…" state instead of looking frozen.

---

## ⚙️ Build & maintenance

- The old **MongoDB migration tool** was removed from the repo and release builds ([#89](https://github.com/0ceanSlim/grain/issues/89)) — grab it from v0.5.4's assets if you still need it.
- The release build now generates the OpenAPI spec before compiling (a missing step that broke the binary build).
- Logging components were audited down to one clean registry; NIP-86 test fixtures regenerated from canonical templates ([#78](https://github.com/0ceanSlim/grain/issues/78)); stale docs cleaned up.
- New standards advertised in NIP-11: **NIP-86** and **NIP-98**.

---

## ⚙️ Migration from v0.6.0

```bash
# Stop your v0.6.0 instance
# Drop in the v0.7.0 binary
# Restart
```

No data migration. nostrdb format unchanged. Everything new is additive.

**To reach the dashboard**, set yourself as owner — either with `GRAIN_OWNER_PUBKEY=npub1...` (or hex) at startup, or by visiting `/setup` and signing in. If your `relay_metadata.json` already has a `pubkey`, that's your admin already.

**DM privacy is on by default.** Unauthenticated `REQ {"kinds":[1059]}` now returns `auth-required` — expected, and almost certainly what you want.

**Backup relays moved to a list.** The single `url:` became `urls:`. Re-enter your backup through the dashboard, or update the YAML:

```yaml
backup_relay:
  enabled: true
  urls:
    - wss://your-backup-relay
```

---

## 🎯 Before vs After

### Before v0.7.0 ❌

- Configuration meant editing YAML by hand and restarting
- No way to manage the relay from a browser
- Login could hang for seconds on every sign-in, and re-prompt the signer on every reload
- The server held plaintext private-key material in the session
- Anyone could pull the entire encrypted-DM social graph
- Your private mute list couldn't reach the blacklist
- One dark theme, one light theme
- Single backup relay

### After v0.7.0 ✅

- A live `/admin` dashboard tunes every setting from the browser, with one-click first-run setup
- Every change is a signed, owner-only operation (NIP-86 over NIP-98) that applies without dropping connections
- Sign-in is instant; signers restore silently across reloads for every method
- The session stores only your public key and method
- DM gift wraps are served only to their recipient — private by default
- Sync your own private mute list to the blacklist without exposing your key
- Seven live-switchable themes on a real design system, plus a restyled Swagger UI
- Multiple backup relays

---

## 🔭 What's Next

**v0.8** turns the relay into an actor on the network — outbox-model routing for bootstrap/index relays ([#56](https://github.com/0ceanSlim/grain/issues/56)) and a keypair-on-first-run flow ([#45](https://github.com/0ceanSlim/grain/issues/45)). Also near-term: geo/region blocking ([#64](https://github.com/0ceanSlim/grain/issues/64)), the nostrdb prefix-filter compliance fix ([#72](https://github.com/0ceanSlim/grain/issues/72)), and configurable NIP-50 indexed kinds ([#71](https://github.com/0ceanSlim/grain/issues/71)).

See `ROADMAP.md` for the full timeline.

🌾

**Full Changelog**: https://github.com/0ceanSlim/grain/compare/v0.6.0...v0.7.0
