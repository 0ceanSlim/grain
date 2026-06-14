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
  // App-relay categories not yet wired (rendered as "coming soon").
  const COMING = [
    ["Indexer", "seed", "Seed relays grain uses to discover everyone's relay lists."],
    ["Broadcast", "local", "Event blasters — forward your posts out to many relays."],
    ["Local", "local", "Same-device / LAN relays, preferred for speed."],
    ["Trusted", "local", "The only relays grain signs NIP-42 AUTH for."],
  ];

  const RM = {
    nip65: [], // [{ url, read, write }]
    lists: { dm: [], search: [], favorites: [], blocked: [] },
    encrypted: {}, // key -> true when the list carries NIP-44 private entries (#100)
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
  // Shown on a NIP-51 list whose event carries NIP-44 private entries grain
  // can't read yet — so it reads as private rather than empty/broken (#100).
  function encryptedHint() {
    return (
      `<div class="flex items-center gap-2 px-3 py-2 text-xs rounded-lg text-text-muted bg-surface-inset">` +
      `🔒 This list has encrypted (private) entries — decrypt support coming.</div>`
    );
  }
  function relayRow(url, removeAttr) {
    return (
      `<div class="flex items-center gap-2.5 px-3 py-2 border rounded-lg bg-surface-elevated border-border">` +
      `<a href="${esc(httpish(url))}" target="_blank" rel="noopener" class="flex-1 min-w-0 text-sm font-medium truncate text-text hover:text-accent hover:underline">${esc(shortRelay(url))}</a>` +
      `<button ${removeAttr} class="px-1 text-lg leading-none shrink-0 text-text-muted hover:text-danger" title="Remove">×</button>` +
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
    if (enc) html += encryptedHint();
    box.innerHTML = html;
    box.querySelectorAll("[data-rm]").forEach((b) => {
      b.onclick = () => {
        const u = b.getAttribute("data-rm");
        RM.lists[key] = RM.lists[key].filter((x) => x !== u);
        renderTagList(key);
      };
    });
  }

  function renderApp() {
    const box = document.getElementById("rm-app-relays");
    if (!box) return;
    box.innerHTML = COMING.map(
      ([title, badge, note]) =>
        `<div class="opacity-60"><div class="flex items-center gap-2">` +
        `<span class="text-sm font-medium text-text">${esc(title)}</span>` +
        `<span class="px-1.5 py-0.5 font-mono text-xs rounded bg-surface-inset-strong text-text-secondary">${esc(badge)}</span>` +
        `<span class="px-2 py-0.5 ml-auto text-xs rounded bg-surface-inset text-text-muted">coming soon</span></div>` +
        `<p class="mt-0.5 text-xs text-text-muted">${esc(note)}</p></div>`
    ).join("");
  }

  function renderAll() {
    renderNip65();
    TAG_CATS.forEach((c) => renderTagList(c.key));
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
    if (window.__rmOverviewTimer) clearInterval(window.__rmOverviewTimer);
    window.__rmOverviewTimer = setInterval(refreshOverview, 5000);

    fetch("/api/v1/user/relay-lists")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (d) {
          RM.nip65 = d.nip65 || [];
          RM.lists.dm = d.dm || [];
          RM.lists.search = d.search || [];
          RM.lists.favorites = d.favorites || [];
          RM.lists.blocked = d.blocked || [];
          RM.encrypted = d.encrypted || {};
        }
        RM.loaded = true;
        renderAll();
      })
      .catch(() => {
        RM.loaded = true;
        renderAll();
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
    const map = { 10050: "dm", 10007: "search", 10012: "favorites", 10006: "blocked" };
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
          renderTagList(key);
        }
      })
      .catch(function () {});
  };
  window.addEventListener("grain:stream", window.__rmStreamHandler);

  init();
})();
