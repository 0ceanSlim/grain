/**
 * Relay manager settings section (#98).
 *
 * Renders the user's relay roles by category and lets them edit + publish each
 * list. NIP-65 (Outbox/Inbox) is one kind-10002 event shown as two views;
 * DM/Search/Favorites/Blocked are NIP-17/51 `relay`-tag lists. The fixed-relay
 * override toggles the outbox model off. Reuses grainPublish (nostr-publish.js)
 * for the live broadcast toast.
 *
 * Data comes from GET /api/v1/user/relay-lists; edits publish via the relay-list
 * build endpoint + the streaming publish endpoint.
 */
(function () {
  const TAG_CATS = [
    { key: "dm", kind: 10050 },
    { key: "search", kind: 10007 },
    { key: "favorites", kind: 10012 },
    { key: "blocked", kind: 10006 },
  ];
  // App relays — local session preferences (seeded from the operator's config),
  // edited here and saved to /api/v1/client/app-relays. Indexer + Broadcast affect
  // routing now; Local + Trusted are stored but inert (Local routing preference and
  // Trusted NIP-42 AUTH are follow-ups).
  const APP_ROLES = [
    { key: "indexer", title: "Indexer", badge: "seed", active: true, note: "Seed relays grain uses to discover everyone's relay lists." },
    { key: "broadcast", title: "Broadcast", badge: "blast", active: true, note: "Every event you publish is also mirrored here — and these are blast/broadcast relays that re-send it onward to many other relays." },
    { key: "local", title: "Local", badge: "local", active: false, note: "Same-device / LAN relays. Stored now; routing preference coming." },
  ];

  const RM = {
    nip65: [], // [{ url, read, write }]
    lists: { dm: [], search: [], favorites: [], blocked: [], private: [] },
    encrypted: {}, // key -> true when the list carries NIP-44 private entries (#100)
    encryptedContent: {}, // key -> raw encrypted blob; the browser decrypts on demand
    decrypted: {}, // key -> [urls] once the user clicks Decrypt (read-only reveal)
    pubkey: "", // session user, for NIP-51 self-decrypt
    app: { indexer: [], broadcast: [], local: [], trusted: [] }, // local-role prefs
    appLoaded: false,
    known: [], // [{ url, connected, pinned, leased }] — the known-relays browser
    knownLoaded: false,
    knownFilter: "",
    knownMap: {}, // url -> { connected, pinned } — status lookup for in-list rows
    knownInfo: {}, // url -> NIP-11 object | null(pending)
    knownExpanded: {}, // url -> true (shared by the browser AND in-list rows)
    knownSort: "relevance", // "relevance" | "ping"
    knownPing: {}, // url -> ms | -1(unreachable) | "pending"
    auth: [], // [{ relay, challenge, authed, at }] — NIP-42 AUTH requests
    authLoaded: false,
    loaded: false,
  };

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }
  function shortRelay(u) {
    return String(u || "").replace(/^wss?:\/\//, "").replace(/\/$/, "");
  }
  function httpish(u) {
    return String(u || "").replace(/^ws(s?):\/\//, "http$1://");
  }
  // Light client-side normalisation for dedupe; the server re-normalises on build.
  function normWs(u) {
    let s = (u || "").trim();
    if (!s) return "";
    if (!/:\/\//.test(s)) s = "wss://" + s;
    try {
      const url = new URL(s);
      if (url.protocol !== "ws:" && url.protocol !== "wss:") return "";
      return url.protocol + "//" + url.host.toLowerCase() + url.pathname.replace(/\/+$/, "");
    } catch (e) {
      return "";
    }
  }
  function setStatus(id, msg) {
    const el = document.getElementById(id);
    if (el) el.textContent = msg || "";
  }

  function spinnerRow(text) {
    return (
      `<div class="flex items-center gap-2 px-3 py-3 text-sm text-text-muted">` +
      `<span class="inline-block w-4 h-4 border-2 rounded-full border-text-secondary border-t-transparent animate-spin"></span>` +
      `${esc(text)}</div>`
    );
  }
  function emptyRow(text) {
    return `<div class="px-3 py-3 text-sm text-center border border-dashed rounded-lg text-text-muted border-border-strong">${esc(text)}</div>`;
  }
  // NIP-51 list `.content` is encrypted to the author themselves; try NIP-44
  // then NIP-04 (per NIP-51). Returns null if neither scheme works. The signer
  // holds the key — grain only ever passes the opaque blob through (#100).
  async function decryptContent(signer, selfPubkey, content) {
    if (signer && signer.nip44 && signer.nip44.decrypt) {
      try {
        return await signer.nip44.decrypt(selfPubkey, content);
      } catch (_) {}
    }
    if (signer && signer.nip04 && signer.nip04.decrypt) {
      try {
        return await signer.nip04.decrypt(selfPubkey, content);
      } catch (_) {}
    }
    return null;
  }
  // Pull relay URLs from a decrypted NIP-51 tag array (["relay", url] / ["r", url]).
  function parseDecryptedRelays(plain) {
    let tags;
    try {
      tags = JSON.parse(plain);
    } catch (_) {
      return null;
    }
    if (!Array.isArray(tags)) return null;
    const out = [];
    for (const t of tags) {
      if (Array.isArray(t) && (t[0] === "relay" || t[0] === "r") && typeof t[1] === "string") {
        const u = normWs(t[1]);
        if (u && out.indexOf(u) < 0) out.push(u);
      }
    }
    return out;
  }
  // The private-entries block for a list whose event carries encrypted content:
  // a Decrypt button, then the revealed entries (read-only) once decrypted (#100).
  // Read-only on purpose — re-encrypting on save needs the encrypt side (later).
  function encryptedSection(key) {
    const dec = RM.decrypted[key];
    if (dec) {
      const rows = dec.length
        ? dec
            .map(
              (u) =>
                `<div class="flex items-center gap-2 px-3 py-2 text-sm border rounded-lg bg-surface-inset border-border">` +
                `<span class="shrink-0" title="Private (encrypted) entry">🔒</span>` +
                `<a href="${esc(httpish(u))}" target="_blank" rel="noopener" class="flex-1 min-w-0 truncate text-text hover:text-accent hover:underline">${esc(shortRelay(u))}</a></div>`
            )
            .join("")
        : `<div class="px-3 py-2 text-xs text-text-muted">No private entries.</div>`;
      return (
        `<div class="mt-1.5 space-y-1.5">` +
        `<div class="px-1 text-xs text-text-muted">🔓 Private entries (read-only — editing comes with the encrypt side):</div>` +
        rows +
        `</div>`
      );
    }
    if (!RM.encryptedContent[key]) {
      return (
        `<div class="flex items-center gap-2 px-3 py-2 text-xs rounded-lg text-text-muted bg-surface-inset">` +
        `🔒 This list has encrypted (private) entries.</div>`
      );
    }
    return (
      `<div class="flex items-center justify-between gap-2 px-3 py-2 text-xs rounded-lg text-text-muted bg-surface-inset">` +
      `<span>🔒 This list has encrypted (private) entries.</span>` +
      `<button data-rm-decrypt="${esc(key)}" class="px-2 py-1 rounded shrink-0 text-text bg-surface-overlay hover:bg-surface-hover">🔓 Decrypt</button>` +
      `</div>`
    );
  }
  // Decrypt a list's private entries with the user's signer and reveal them.
  async function rmDecrypt(key) {
    const content = RM.encryptedContent[key];
    if (!content) return;
    if (typeof window.restoreSigner === "function") {
      try {
        await window.restoreSigner();
      } catch (_) {}
    }
    const signer = window.grainSigner;
    if (!signer) {
      setStatus("rm-" + key + "-status", "Signer unavailable — reconnect to decrypt.");
      return;
    }
    setStatus("rm-" + key + "-status", "Decrypting…");
    const plain = await decryptContent(signer, RM.pubkey, content);
    if (plain == null) {
      setStatus("rm-" + key + "-status", "Couldn't decrypt — signer declined or no NIP-44 support.");
      return;
    }
    const urls = parseDecryptedRelays(plain);
    if (urls == null) {
      setStatus("rm-" + key + "-status", "Decrypted, but couldn't parse the entries.");
      return;
    }
    RM.decrypted[key] = urls;
    setStatus("rm-" + key + "-status", "");
    renderTagList(key);
  }
  // Status dot shared by in-list rows and the known-relays browser.
  function dotMarkup(connected, pinned) {
    const cls = connected ? "bg-success" : pinned ? "bg-accent" : "bg-text-muted";
    const title = connected ? "connected" : pinned ? "pinned" : "known";
    return `<span class="w-2 h-2 rounded-full shrink-0 ${cls}" title="${title}"></span>`;
  }
  // Status dot for an in-list relay, looked up from the known-relays pool status.
  function relayStatusDot(url) {
    const st = RM.knownMap[normWs(url)] || RM.knownMap[url] || {};
    return dotMarkup(st.connected, st.pinned);
  }
  // The NIP-11 expander — a labelled pill button (not a bare chevron) so it's
  // clear the row opens for more info.
  function infoToggleBtn(url, expanded) {
    return (
      `<button data-relay-info="${esc(url)}" title="${expanded ? "Hide" : "Show"} relay info (NIP-11)" ` +
      `class="flex items-center gap-1 px-1.5 py-0.5 text-xs font-medium border rounded shrink-0 border-border bg-surface-overlay text-text-secondary hover:text-text hover:bg-surface-hover">` +
      `<span>info</span><span class="text-[0.65rem] leading-none">${expanded ? "▲" : "▼"}</span></button>`
    );
  }
  // The NIP-11 detail block for an expanded relay row — shared everywhere.
  // Lazily filled by rmRelayInfoToggle.
  function relayInfoDetailHTML(url) {
    if (!RM.knownExpanded[url]) return "";
    const info = RM.knownInfo[url];
    return info === null
      ? `<div class="px-3 pb-2 text-xs text-text-muted">Loading NIP-11…</div>`
      : knownNip11(info);
  }
  // Toggle a relay's NIP-11 detail (shared by in-list rows + the browser).
  // Fetches lazily, then re-renders both so the row updates wherever it appears.
  async function rmRelayInfoToggle(url) {
    RM.knownExpanded[url] = !RM.knownExpanded[url];
    if (RM.knownExpanded[url] && RM.knownInfo[url] === undefined) {
      RM.knownInfo[url] = null; // pending
      renderAll();
      renderKnown();
      try {
        const r = await fetch("/api/v1/relay-info?url=" + encodeURIComponent(url));
        RM.knownInfo[url] = r.ok ? await r.json() : {};
      } catch (_) {
        RM.knownInfo[url] = {};
      }
    }
    renderAll();
    renderKnown();
  }

  function relayRow(url, removeAttr) {
    const expanded = !!RM.knownExpanded[url];
    return (
      `<div class="border rounded-lg bg-surface-elevated border-border">` +
      `<div class="flex items-center gap-2.5 px-3 py-2">` +
      relayStatusDot(url) +
      `<a href="${esc(httpish(url))}" target="_blank" rel="noopener" class="flex-1 min-w-0 text-sm font-medium truncate text-text hover:text-accent hover:underline">${esc(shortRelay(url))}</a>` +
      infoToggleBtn(url, expanded) +
      `<button ${removeAttr} class="px-1 text-lg leading-none shrink-0 text-text-muted hover:text-danger" title="Remove">×</button>` +
      `</div>` +
      relayInfoDetailHTML(url) +
      `</div>`
    );
  }

  function renderNip65() {
    const ob = document.getElementById("rm-outbox-list");
    const ib = document.getElementById("rm-inbox-list");
    if (!ob || !ib) return;
    if (!RM.loaded) {
      ob.innerHTML = spinnerRow("Fetching your relays…");
      ib.innerHTML = spinnerRow("Fetching your relays…");
      return;
    }
    const outbox = RM.nip65.filter((e) => e.write);
    const inbox = RM.nip65.filter((e) => e.read);
    ob.innerHTML = outbox.length
      ? outbox.map((e) => relayRow(e.url, `data-ob="${esc(e.url)}"`)).join("")
      : emptyRow("No outbox relays yet.");
    ib.innerHTML = inbox.length
      ? inbox.map((e) => relayRow(e.url, `data-ib="${esc(e.url)}"`)).join("")
      : emptyRow("No inbox relays yet.");
    ob.querySelectorAll("[data-ob]").forEach((b) => {
      b.onclick = () => nip65Remove(b.getAttribute("data-ob"), "write");
    });
    ib.querySelectorAll("[data-ib]").forEach((b) => {
      b.onclick = () => nip65Remove(b.getAttribute("data-ib"), "read");
    });
    wireRelayInfo(ob);
    wireRelayInfo(ib);
  }
  // Wire the ▾ NIP-11 expanders within a list container.
  function wireRelayInfo(box) {
    box.querySelectorAll("[data-relay-info]").forEach((b) => {
      b.onclick = () => rmRelayInfoToggle(b.getAttribute("data-relay-info"));
    });
  }

  function renderTagList(key) {
    const box = document.getElementById("rm-" + key + "-list");
    if (!box) return;
    if (!RM.loaded) {
      box.innerHTML = spinnerRow("Fetching…");
      return;
    }
    const list = RM.lists[key];
    const enc = RM.encrypted && RM.encrypted[key];
    let html = list.length
      ? list.map((u) => relayRow(u, `data-rm="${esc(u)}"`)).join("")
      : enc
      ? ""
      : emptyRow("None yet.");
    if (enc) html += encryptedSection(key);
    box.innerHTML = html;
    box.querySelectorAll("[data-rm]").forEach((b) => {
      b.onclick = () => {
        const u = b.getAttribute("data-rm");
        RM.lists[key] = RM.lists[key].filter((x) => x !== u);
        renderTagList(key);
      };
    });
    box.querySelectorAll("[data-rm-decrypt]").forEach((b) => {
      b.onclick = () => rmDecrypt(b.getAttribute("data-rm-decrypt"));
    });
    wireRelayInfo(box);
  }

  function appRow(key, url) {
    return (
      `<div class="flex items-center gap-2.5 px-3 py-2 border rounded-lg bg-surface-elevated border-border">` +
      `<a href="${esc(httpish(url))}" target="_blank" rel="noopener" class="flex-1 min-w-0 text-sm font-medium truncate text-text hover:text-accent hover:underline">${esc(shortRelay(url))}</a>` +
      `<button data-app-remove="${esc(url)}" data-app-key="${esc(key)}" class="px-1 text-lg leading-none shrink-0 text-text-muted hover:text-danger" title="Remove">×</button>` +
      `</div>`
    );
  }

  function renderApp() {
    const box = document.getElementById("rm-app-relays");
    if (!box) return;
    if (!RM.appLoaded) {
      box.innerHTML = spinnerRow("Fetching…");
      return;
    }
    box.innerHTML =
      APP_ROLES.map((rr) => {
        const list = RM.app[rr.key] || [];
        const rows = list.length
          ? list.map((u) => appRow(rr.key, u)).join("")
          : `<div class="px-3 py-2 text-xs text-text-muted">None.</div>`;
        return (
          `<div class="${rr.active ? "" : "opacity-70"}">` +
          `<div class="flex items-center gap-2">` +
          `<span class="text-sm font-medium text-text">${esc(rr.title)}</span>` +
          `<span class="px-1.5 py-0.5 font-mono text-xs rounded bg-surface-inset-strong text-text-secondary">${esc(rr.badge)}</span>` +
          (rr.active ? "" : `<span class="px-2 py-0.5 ml-auto text-xs rounded bg-surface-inset text-text-muted">stored · inert</span>`) +
          `</div>` +
          `<p class="mt-0.5 mb-1.5 text-xs text-text-muted">${esc(rr.note)}</p>` +
          `<div class="space-y-1.5">${rows}</div>` +
          `<div class="flex gap-2 mt-2">` +
          `<input data-app-input="${esc(rr.key)}" type="text" placeholder="Add a relay…" list="rm-known-options" class="flex-1 px-3 py-2 text-sm border rounded-lg bg-surface-elevated text-text border-border" />` +
          `<button data-app-add="${esc(rr.key)}" class="px-3 py-2 text-sm rounded-lg text-text bg-surface-elevated hover:bg-surface-hover">+ Add</button>` +
          `</div></div>`
        );
      }).join("") +
      `<div class="flex items-center justify-between gap-3 pt-3 mt-1 border-t border-border">` +
      `<span class="text-xs text-text-muted">Session preferences — saved to grain, not published as events.</span>` +
      `<button onclick="rmAppSave()" class="px-4 py-2 text-sm rounded-lg text-text-on-accent bg-accent hover:opacity-80">Save</button>` +
      `</div>` +
      `<p id="rm-app-status" class="mt-1 text-xs text-right text-text-secondary"></p>`;
    box.querySelectorAll("[data-app-add]").forEach((b) => {
      b.onclick = () => {
        const key = b.getAttribute("data-app-add");
        const inp = box.querySelector('[data-app-input="' + key + '"]');
        if (inp && inp.value.trim()) {
          rmAppAdd(key, inp.value);
          inp.value = "";
        }
      };
    });
    box.querySelectorAll("[data-app-remove]").forEach((b) => {
      b.onclick = () => rmAppRemove(b.getAttribute("data-app-key"), b.getAttribute("data-app-remove"));
    });
  }

  function rmAppAdd(key, url) {
    const u = normWs(url);
    if (!u || !RM.app[key]) return;
    if (RM.app[key].indexOf(u) < 0) RM.app[key].push(u);
    renderApp();
  }
  function rmAppRemove(key, url) {
    if (!RM.app[key]) return;
    RM.app[key] = RM.app[key].filter((x) => x !== url);
    renderApp();
  }
  async function rmAppSave() {
    setStatus("rm-app-status", "Saving…");
    try {
      const resp = await fetch("/api/v1/client/app-relays", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(RM.app),
      });
      if (!resp.ok) {
        setStatus("rm-app-status", "Save failed: " + (await resp.text()));
        return;
      }
      const d = await resp.json();
      RM.app = {
        indexer: d.indexer || [],
        broadcast: d.broadcast || [],
        local: d.local || [],
        trusted: d.trusted || [],
      };
      setStatus("rm-app-status", "✓ Saved.");
      renderApp();
    } catch (e) {
      setStatus("rm-app-status", "Save failed.");
    }
  }
  window.rmAppSave = rmAppSave;

  // ── NIP-42 AUTH requests ──────────────────────────────────────────────
  // Relays that challenged grain for AUTH this session. Authenticate signs a
  // kind-22242 event with the browser signer; grain relays it on the same
  // connection. A relay stays trusted for the session once answered.

  function authRow(a) {
    const dotCls = a.authed ? "bg-success" : "bg-warning";
    const status = a.authed
      ? `<span class="px-1.5 py-0.5 text-xs rounded shrink-0 bg-surface-inset text-success">trusted · session</span>`
      : `<span class="px-1.5 py-0.5 text-xs rounded shrink-0 bg-surface-inset text-warning">challenged</span>`;
    const authBtn = a.authed
      ? ""
      : `<button data-auth-go="${esc(a.relay)}" class="px-2.5 py-1 text-xs rounded shrink-0 text-text-on-accent bg-accent hover:opacity-80">Authenticate</button>`;
    return (
      `<div class="flex items-center gap-2 px-3 py-2 border rounded-lg bg-surface-elevated border-border">` +
      `<span class="w-2 h-2 rounded-full shrink-0 ${dotCls}" title="${a.authed ? "authed" : "challenged"}"></span>` +
      `<a href="${esc(httpish(a.relay))}" target="_blank" rel="noopener" class="flex-1 min-w-0 text-sm truncate text-text hover:text-accent hover:underline">${esc(shortRelay(a.relay))}</a>` +
      status +
      authBtn +
      `<button data-auth-remove="${esc(a.relay)}" class="px-1 text-lg leading-none shrink-0 text-text-muted hover:text-danger" title="Forget for this session">×</button>` +
      `</div>`
    );
  }

  function renderAuth() {
    const box = document.getElementById("rm-auth-list");
    if (!box) return;
    if (!RM.authLoaded) {
      box.innerHTML = spinnerRow("Checking…");
      return;
    }
    if (!RM.auth.length) {
      box.innerHTML = emptyRow("No relays have requested authentication this session.");
      return;
    }
    box.innerHTML = RM.auth
      .slice()
      .sort((a, b) => Number(a.authed) - Number(b.authed) || a.relay.localeCompare(b.relay))
      .map(authRow)
      .join("");
    box.querySelectorAll("[data-auth-go]").forEach((b) => {
      b.onclick = () => rmAuthenticate(b.getAttribute("data-auth-go"));
    });
    box.querySelectorAll("[data-auth-remove]").forEach((b) => {
      b.onclick = () => rmAuthRemove(b.getAttribute("data-auth-remove"));
    });
  }

  function rmAuthRefresh() {
    fetch("/api/v1/client/auth-requests")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        RM.auth = Array.isArray(d) ? d : [];
        RM.authLoaded = true;
        renderAuth();
      })
      .catch(() => {
        RM.authLoaded = true;
        renderAuth();
      });
  }

  async function rmAuthenticate(relay) {
    const a = RM.auth.find((x) => x.relay === relay);
    if (!a || !a.challenge) {
      rmAuthRefresh();
      return;
    }
    if (typeof window.restoreSigner === "function") {
      try {
        await window.restoreSigner();
      } catch (_) {}
    }
    const signer = window.grainSigner;
    if (!signer || typeof signer.signEvent !== "function") {
      setStatus("rm-auth-status", "Signer unavailable — reconnect to authenticate.");
      return;
    }
    setStatus("rm-auth-status", "Signing AUTH for " + shortRelay(relay) + "…");
    let signed;
    try {
      signed = await signer.signEvent({
        kind: 22242,
        created_at: Math.floor(Date.now() / 1000),
        content: "",
        tags: [
          ["relay", relay],
          ["challenge", a.challenge],
        ],
      });
    } catch (_) {
      setStatus("rm-auth-status", "Signing declined.");
      return;
    }
    try {
      const resp = await fetch("/api/v1/client/auth", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ relay: relay, event: signed }),
      });
      const d = resp.ok ? await resp.json() : {};
      setStatus(
        "rm-auth-status",
        d.success ? "✓ Authenticated to " + shortRelay(relay) + "." : "AUTH failed: " + (d.error || "relay unreachable")
      );
    } catch (_) {
      setStatus("rm-auth-status", "AUTH request failed.");
    }
    rmAuthRefresh();
  }

  function rmAuthRemove(relay) {
    fetch("/api/v1/client/auth/remove", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ relay: relay }),
    })
      .catch(function () {})
      .finally(rmAuthRefresh);
  }

  // ── Known-relays browser ──────────────────────────────────────────────
  // Browse every relay grain has seen (config seeds + your mailboxes + ones
  // it's connected to), with live pool status and NIP-11 on expand, and stage
  // any of them into your lists (then Save & sign that list to publish).

  // Which of the user's lists a relay is currently in — for the row badges.
  function knownInLists(url) {
    const u = normWs(url);
    const out = [];
    const e = RM.nip65.find((x) => x.url === u);
    if (e && e.write) out.push("outbox");
    if (e && e.read) out.push("inbox");
    ["search", "favorites", "blocked", "dm"].forEach((k) => {
      if ((RM.lists[k] || []).indexOf(u) >= 0) out.push(k);
    });
    return out;
  }

  // Fill the shared <datalist> that every "add a relay" input autocompletes
  // from. Connected/pinned float up so the native dropdown's top matches are the
  // useful ones.
  function populateKnownDatalist() {
    const dl = document.getElementById("rm-known-options");
    if (!dl) return;
    dl.innerHTML = RM.known
      .slice()
      .sort((a, b) => {
        const ra = a.connected ? 0 : a.pinned ? 1 : 2;
        const rb = b.connected ? 0 : b.pinned ? 1 : 2;
        return ra - rb || a.url.localeCompare(b.url);
      })
      .map((k) => `<option value="${esc(k.url)}"></option>`)
      .join("");
  }

  function knownNip11(info) {
    if (!info || !Object.keys(info).length)
      return `<div class="px-3 pb-2 text-xs text-text-muted">No NIP-11 info advertised.</div>`;
    const nips = (info.supported_nips || []).join(", ");
    const lim = info.limitation || {};
    const flags = [];
    if (lim.auth_required) flags.push("🔒 AUTH required");
    if (lim.payment_required) flags.push("💰 paid");
    if (lim.restricted_writes) flags.push("✍ restricted writes");
    const rows = [];
    if (info.name) rows.push(`<div class="font-medium text-text">${esc(info.name)}</div>`);
    if (info.description) rows.push(`<div>${esc(info.description)}</div>`);
    if (info.software)
      rows.push(
        `<div><span class="text-text-muted">software</span> ${esc(info.software)}${info.version ? " · " + esc(info.version) : ""}</div>`
      );
    if (nips) rows.push(`<div><span class="text-text-muted">NIPs</span> ${esc(nips)}</div>`);
    if (flags.length) rows.push(`<div class="text-warning">${esc(flags.join("  ·  "))}</div>`);
    return `<div class="px-3 pb-2.5 space-y-0.5 text-xs text-text-secondary">${rows.join("")}</div>`;
  }

  // A latency badge (TCP-connect ms) once a relay has been pinged — green/amber/
  // red by speed, "—" if unreachable, a spinner while in flight.
  function pingBadge(url) {
    const p = RM.knownPing[url];
    if (p === undefined) return "";
    if (p === "pending")
      return `<span class="inline-block w-3 h-3 border rounded-full shrink-0 border-text-secondary border-t-transparent animate-spin" title="pinging…"></span>`;
    if (p === -1)
      return `<span class="text-xs shrink-0 text-text-muted" title="unreachable">—</span>`;
    const cls = p < 150 ? "text-success" : p < 500 ? "text-warning" : "text-danger";
    return `<span class="font-mono text-xs shrink-0 ${cls}" title="TCP connect latency">${p}ms</span>`;
  }

  function knownRow(k) {
    const expanded = !!RM.knownExpanded[k.url];
    const badges = knownInLists(k.url)
      .map(
        (l) =>
          `<span class="px-1.5 py-0.5 text-xs rounded bg-surface-inset text-text-secondary">${esc(l)}</span>`
      )
      .join("");
    return (
      `<div class="border rounded-lg bg-surface-elevated border-border">` +
      `<div class="flex items-center gap-2 px-3 py-2">` +
      dotMarkup(k.connected, k.pinned) +
      `<a href="${esc(httpish(k.url))}" target="_blank" rel="noopener" class="flex-1 min-w-0 text-sm truncate text-text hover:text-accent hover:underline" title="${esc(k.url)}">${esc(shortRelay(k.url))}</a>` +
      pingBadge(k.url) +
      badges +
      `<select data-known-add="${esc(k.url)}" class="px-1 py-1 text-xs border rounded shrink-0 bg-surface-overlay text-text border-border">` +
      `<option value="">+ list…</option>` +
      `<option value="outbox">Outbox</option><option value="inbox">Inbox</option>` +
      `<option value="search">Search</option><option value="favorites">Favorites</option>` +
      `<option value="blocked">Blocked</option><option value="dm">DM</option>` +
      `</select>` +
      infoToggleBtn(k.url, expanded) +
      `</div>` +
      relayInfoDetailHTML(k.url) +
      `</div>`
    );
  }

  // Ping-sort rank: measured latency first (ascending), unknown/pending in the
  // middle, unreachable last.
  function pingRank(url) {
    const p = RM.knownPing[url];
    if (typeof p === "number") return p < 0 ? 2e6 : p;
    return 1e6;
  }

  // The filtered known set in the active sort order — shared by render and the
  // ping pass so "visible" means the same thing in both.
  function knownOrdered() {
    const q = (RM.knownFilter || "").toLowerCase();
    const filtered = q
      ? RM.known.filter((k) => k.url.toLowerCase().indexOf(q) >= 0)
      : RM.known;
    if (RM.knownSort === "ping") {
      return filtered
        .slice()
        .sort((a, b) => pingRank(a.url) - pingRank(b.url) || a.url.localeCompare(b.url));
    }
    // Relevance-first: connected, then pinned, then ones already in your lists,
    // then the rest alphabetically — so the top of 900+ is actually useful.
    const ranked = filtered.map((k) => ({
      k,
      r: k.connected ? 0 : k.pinned ? 1 : knownInLists(k.url).length ? 2 : 3,
    }));
    ranked.sort((a, b) => a.r - b.r || a.k.url.localeCompare(b.k.url));
    return ranked.map((x) => x.k);
  }

  const KNOWN_CAP = 150; // rows rendered at once
  const PING_BATCH = 50; // relays pinged per pass

  function renderKnown() {
    const box = document.getElementById("rm-known-list");
    if (!box) return;
    if (!RM.knownLoaded) {
      box.innerHTML = spinnerRow("Loading known relays…");
      return;
    }
    const ordered = knownOrdered();
    if (!ordered.length) {
      box.innerHTML = emptyRow(RM.knownFilter ? "No matches." : "No known relays yet.");
      return;
    }
    let html = ordered.slice(0, KNOWN_CAP).map(knownRow).join("");
    if (ordered.length > KNOWN_CAP)
      html += `<div class="px-3 py-1.5 text-xs text-text-muted">+${ordered.length - KNOWN_CAP} more — type to filter.</div>`;
    box.innerHTML = html;
    wireRelayInfo(box); // shared ▾ NIP-11 toggles
    box.querySelectorAll("[data-known-add]").forEach((s) => {
      s.onchange = () => {
        rmKnownAdd(s.getAttribute("data-known-add"), s.value);
        s.value = "";
      };
    });
  }

  // Ping the relays currently in view we haven't measured yet, then re-render
  // (which re-sorts when in "fastest" mode).
  async function rmKnownPingVisible() {
    const urls = knownOrdered()
      .slice(0, PING_BATCH)
      .map((k) => k.url)
      .filter((u) => RM.knownPing[u] === undefined);
    if (!urls.length) return;
    urls.forEach((u) => (RM.knownPing[u] = "pending"));
    renderKnown();
    try {
      const r = await fetch("/api/v1/relays/ping", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ urls }),
      });
      const d = r.ok ? await r.json() : {};
      urls.forEach((u) => (RM.knownPing[u] = typeof d[u] === "number" ? d[u] : -1));
    } catch (_) {
      urls.forEach((u) => (RM.knownPing[u] = -1));
    }
    renderKnown();
  }

  function rmKnownAdd(url, target) {
    if (!target) return;
    if (target === "outbox" || target === "inbox") {
      nip65Upsert(url, target === "outbox" ? "write" : "read"); // re-renders NIP-65
    } else if (RM.lists[target]) {
      const u = normWs(url);
      if (u && RM.lists[target].indexOf(u) < 0) {
        RM.lists[target].push(u);
        renderTagList(target);
      }
    }
    renderKnown(); // refresh the which-lists badges on this row
  }

  window.rmKnownSort = function (v) {
    RM.knownSort = v === "ping" ? "ping" : "relevance";
    renderKnown();
    if (RM.knownSort === "ping") rmKnownPingVisible();
  };

  window.rmKnownSearch = function (v) {
    RM.knownFilter = v || "";
    renderKnown();
    if (RM.knownSort === "ping") rmKnownPingVisible();
  };

  // The Private (10013, NIP-37) list is read-only: NIP-37 keeps relays in the
  // encrypted content (public entries are rare), so this mostly shows the
  // decrypt reveal. Editing/re-encrypting waits for the encrypt side.
  function renderPrivate() {
    const box = document.getElementById("rm-private-list");
    if (!box) return;
    if (!RM.loaded) {
      box.innerHTML = spinnerRow("Fetching…");
      return;
    }
    const pub = RM.lists.private || [];
    const enc = RM.encrypted && RM.encrypted.private;
    let html = pub
      .map(
        (u) =>
          `<div class="flex items-center gap-2 px-3 py-2 text-sm border rounded-lg bg-surface-elevated border-border">` +
          `<a href="${esc(httpish(u))}" target="_blank" rel="noopener" class="flex-1 min-w-0 truncate text-text hover:text-accent hover:underline">${esc(shortRelay(u))}</a></div>`
      )
      .join("");
    if (enc) html += encryptedSection("private");
    else if (!pub.length) html += emptyRow("No private relay list.");
    box.innerHTML = html;
    box.querySelectorAll("[data-rm-decrypt]").forEach((b) => {
      b.onclick = () => rmDecrypt(b.getAttribute("data-rm-decrypt"));
    });
  }

  function renderAll() {
    renderNip65();
    TAG_CATS.forEach((c) => renderTagList(c.key));
    renderPrivate();
  }

  function nip65Upsert(url, field) {
    const u = normWs(url);
    if (!u) return;
    let e = RM.nip65.find((x) => x.url === u);
    if (!e) {
      e = { url: u, read: false, write: false };
      RM.nip65.push(e);
    }
    e[field] = true;
    renderNip65();
  }
  function nip65Remove(url, field) {
    const u = normWs(url);
    const e = RM.nip65.find((x) => x.url === u);
    if (!e) return;
    e[field] = false;
    if (!e.read && !e.write) RM.nip65 = RM.nip65.filter((x) => x.url !== u);
    renderNip65();
  }

  async function publishRelayList(kind, entries, statusId) {
    if (!window.grainPublish) {
      setStatus(statusId, "Publish helper not loaded — reload the page.");
      return;
    }
    setStatus(statusId, "Building your list…");
    let resp;
    try {
      resp = await fetch("/api/v1/user/relay-list/build", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ kind: kind, entries: entries }),
      });
    } catch (e) {
      setStatus(statusId, "Network error.");
      return;
    }
    if (!resp.ok) {
      setStatus(statusId, "Couldn't build the list: " + (await resp.text()));
      return;
    }
    const unsigned = await resp.json();
    const res = await window.grainPublish.signAndPublish(unsigned, { title: "Publishing relay list…" });
    if (res && res.accepted) setStatus(statusId, "✓ Saved — published to your relays.");
    else if (res) setStatus(statusId, "Signed, but no relay accepted it yet — see the toast.");
    else setStatus(statusId, "Publish failed — see the toast or the browser console.");
  }

  // Public handlers (referenced from settings.html).
  window.rmAddOutbox = function () {
    const i = document.getElementById("rm-outbox-input");
    if (i && i.value.trim()) {
      nip65Upsert(i.value, "write");
      i.value = "";
    }
  };
  window.rmAddInbox = function () {
    const i = document.getElementById("rm-inbox-input");
    if (i && i.value.trim()) {
      nip65Upsert(i.value, "read");
      i.value = "";
    }
  };
  window.rmAdd = function (key) {
    const i = document.getElementById("rm-" + key + "-input");
    if (!i || !i.value.trim()) return;
    const u = normWs(i.value);
    if (u && RM.lists[key].indexOf(u) < 0) RM.lists[key].push(u);
    i.value = "";
    renderTagList(key);
  };
  window.rmSaveNip65 = function () {
    publishRelayList(
      10002,
      RM.nip65.map((e) => ({ url: e.url, read: e.read, write: e.write })),
      "rm-nip65-status"
    );
  };
  window.rmSave = function (key) {
    const cat = TAG_CATS.find((c) => c.key === key);
    if (!cat) return;
    publishRelayList(cat.kind, RM.lists[key].map((u) => ({ url: u })), "rm-" + key + "-status");
  };
  window.rmToggleFixed = async function () {
    const cb = document.getElementById("rm-fixed-toggle");
    const inputs = document.getElementById("rm-fixed-inputs");
    if (inputs) inputs.classList.toggle("hidden", !cb.checked);
    if (!cb.checked) {
      try {
        await fetch("/api/v1/client/fixed-relays", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ enabled: false }),
        });
        setStatus("rm-fixed-status", "Outbox model restored.");
      } catch (e) {}
    }
  };
  window.rmApplyFixed = async function () {
    const split = (id) =>
      (document.getElementById(id).value || "").split(/[\s,]+/).map(normWs).filter(Boolean);
    try {
      const resp = await fetch("/api/v1/client/fixed-relays", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: true, read: split("rm-fixed-read"), write: split("rm-fixed-write") }),
      });
      setStatus("rm-fixed-status", resp.ok ? "Fixed relays applied — outbox model OFF." : "Failed to apply.");
    } catch (e) {
      setStatus("rm-fixed-status", "Network error.");
    }
  };

  function refreshOverview() {
    fetch("/api/v1/client/status")
      .then((r) => (r.ok ? r.json() : null))
      .then((s) => {
        if (!s || !s.initialized) return;
        const box = document.getElementById("rm-overview");
        if (!box) return;
        const cells = [
          ["Known", s.pool_known || 0, "text-text"],
          ["Connected", s.pool_connected || 0, "text-success"],
          ["In use", s.pool_leased || 0, "text-text"],
          ["Pinned", s.pool_pinned || 0, "text-text"],
        ];
        box.innerHTML = cells
          .map(
            ([l, v, c]) =>
              `<div class="px-3 py-2 rounded-lg bg-surface-elevated"><div class="text-xs text-text-secondary">${l}</div>` +
              `<div class="text-xl font-semibold ${c}">${v}</div></div>`
          )
          .join("");
      })
      .catch(() => {});
  }

  function init() {
    if (!document.getElementById("relay-manager-section")) return;
    renderApp();
    renderAll(); // spinners while loaded === false
    refreshOverview();
    renderAuth(); // initial spinner
    rmAuthRefresh();
    if (window.__rmOverviewTimer) clearInterval(window.__rmOverviewTimer);
    window.__rmOverviewTimer = setInterval(() => {
      refreshOverview();
      rmAuthRefresh(); // AUTH challenges arrive async during pool activity
    }, 5000);

    fetch("/api/v1/user/relay-lists")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (d) {
          RM.nip65 = d.nip65 || [];
          RM.lists.dm = d.dm || [];
          RM.lists.search = d.search || [];
          RM.lists.favorites = d.favorites || [];
          RM.lists.blocked = d.blocked || [];
          RM.lists.private = d.private || [];
          RM.encrypted = d.encrypted || {};
          RM.encryptedContent = d.encrypted_content || {};
          RM.pubkey = d.pubkey || "";
        }
        RM.loaded = true;
        renderAll();
      })
      .catch(() => {
        RM.loaded = true;
        renderAll();
      });

    fetch("/api/v1/client/app-relays")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (d) {
          RM.app = {
            indexer: d.indexer || [],
            broadcast: d.broadcast || [],
            local: d.local || [],
            trusted: d.trusted || [],
          };
        }
        RM.appLoaded = true;
        renderApp();
      })
      .catch(() => {
        RM.appLoaded = true;
        renderApp();
      });

    renderKnown(); // initial spinner
    fetch("/api/v1/client/known-relays")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        RM.known = Array.isArray(d) ? d : [];
        RM.knownMap = {};
        RM.known.forEach(
          (k) => (RM.knownMap[k.url] = { connected: k.connected, pinned: k.pinned })
        );
        RM.knownLoaded = true;
        populateKnownDatalist(); // autocomplete source for every add-relay input
        renderKnown();
        renderAll(); // in-list rows now pick up their status dots
      })
      .catch(() => {
        RM.knownLoaded = true;
        renderKnown();
      });
  }

  // Live-sync (#87): when one of our relay lists changes — here or in another
  // client — re-pull just that list so the page stays current without a reload.
  // Re-wire on each load (the handler closes over this module's state).
  if (window.__rmStreamHandler)
    window.removeEventListener("grain:stream", window.__rmStreamHandler);
  window.__rmStreamHandler = function (e) {
    const m = e.detail || {};
    if (m.type !== "list-updated") return;
    const map = { 10050: "dm", 10007: "search", 10012: "favorites", 10006: "blocked", 10013: "private" };
    if (m.kind !== 10002 && !map[m.kind]) return;
    fetch("/api/v1/user/relay-lists")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (!d) return;
        if (m.kind === 10002) {
          RM.nip65 = d.nip65 || [];
          renderNip65();
        } else {
          const key = map[m.kind];
          RM.lists[key] = d[key] || [];
          RM.encrypted = d.encrypted || RM.encrypted;
          RM.encryptedContent = d.encrypted_content || RM.encryptedContent;
          delete RM.decrypted[key]; // changed elsewhere — re-show the Decrypt button
          if (key === "private") renderPrivate();
          else renderTagList(key);
        }
      })
      .catch(function () {});
  };
  window.addEventListener("grain:stream", window.__rmStreamHandler);

  init();
})();
