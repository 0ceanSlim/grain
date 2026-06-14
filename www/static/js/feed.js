/**
 * Live relay event stream (homepage).
 *
 * grain IS a nostr relay, so the browser connects to it directly over WebSocket,
 * asks for recent + live events of all kinds, and renders a compact feed: author
 * avatar, readable kind label, relative time, and (for kind 1) a text snippet.
 * Click a row to open the event page; click the avatar to open the author's
 * profile. The row UI is the reusable part — an outbox feed (the #77 streaming
 * primitive, many relays) would swap the source but reuse this rendering.
 */
(function () {
  const LIMIT = 50; // recent events to backfill
  const MAX_ROWS = 100; // cap the DOM
  const SNIPPET = 140; // kind-1 text cut (the classic 140)

  let kinds = {};
  const seen = {};
  const profiles = {}; // hex pubkey -> { picture, name } | "pending"

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }
  function cssEsc(s) {
    return String(s).replace(/["\\]/g, "\\$&");
  }
  function kindName(k) {
    return kinds[String(k)] || "Kind " + k;
  }
  function relTime(ts) {
    const s = Math.floor(Date.now() / 1000) - (ts || 0);
    if (s < 5) return "just now";
    if (s < 60) return s + "s";
    if (s < 3600) return Math.floor(s / 60) + "m";
    if (s < 86400) return Math.floor(s / 3600) + "h";
    if (s < 2592000) return Math.floor(s / 86400) + "d";
    return new Date((ts || 0) * 1000).toLocaleDateString();
  }
  function snippet(content) {
    let t = (content || "").replace(/\s+/g, " ").trim();
    if (t.length > SNIPPET) t = t.slice(0, SNIPPET) + "…";
    return t;
  }
  function avatar(pubkey) {
    const p = profiles[pubkey];
    if (p && p.picture) {
      return `<img src="${esc(p.picture)}" alt="" class="object-cover w-8 h-8 rounded-full shrink-0 bg-surface-elevated" />`;
    }
    return `<div class="flex items-center justify-center w-8 h-8 text-xs rounded-full shrink-0 bg-surface-elevated text-text-muted">${esc((pubkey || "").slice(0, 2))}</div>`;
  }

  function init() {
    const box = document.getElementById("relay-stream-list");
    if (!box) return;
    box.innerHTML = `<div class="px-3 py-4 text-sm text-text-muted">Connecting to this relay…</div>`;
    fetch("/api/v1/kinds")
      .then((r) => (r.ok ? r.json() : {}))
      .then((m) => {
        kinds = m || {};
      })
      .catch(() => {})
      .finally(() => connect());
  }

  function connect() {
    closeWs();
    const box = document.getElementById("relay-stream-list");
    if (!box) return;
    const proto = location.protocol === "https:" ? "wss://" : "ws://";
    let sock;
    try {
      sock = new WebSocket(proto + location.host);
    } catch (e) {
      box.innerHTML = `<div class="px-3 py-4 text-sm text-danger">Couldn't connect to the relay.</div>`;
      return;
    }
    window.__feedWs = sock;
    const sub = "feed-" + Math.random().toString(36).slice(2, 8);
    let got = false;

    sock.onopen = function () {
      sock.send(JSON.stringify(["REQ", sub, { limit: LIMIT }]));
    };
    sock.onmessage = function (e) {
      let msg;
      try {
        msg = JSON.parse(e.data);
      } catch (_) {
        return;
      }
      if (msg[0] === "EVENT" && msg[2]) {
        if (!got) {
          got = true;
          box.innerHTML = "";
        }
        addEvent(box, msg[2]);
      } else if (msg[0] === "EOSE" && !got) {
        box.innerHTML = `<div class="px-3 py-4 text-sm text-text-muted">No recent events on this relay yet — new ones will appear here.</div>`;
      }
    };
    sock.onclose = function () {
      // Reconnect while the feed is still on screen.
      if (window.__feedWs === sock) {
        window.__feedWs = null;
        setTimeout(function () {
          if (document.getElementById("relay-stream-list")) connect();
        }, 3000);
      }
    };
    sock.onerror = function () {};
  }
  function closeWs() {
    if (window.__feedWs) {
      try {
        window.__feedWs.close();
      } catch (e) {}
      window.__feedWs = null;
    }
  }

  function addEvent(box, ev) {
    if (!ev || !ev.id || seen[ev.id]) return;
    seen[ev.id] = true;
    box.insertAdjacentHTML("afterbegin", row(ev));
    while (box.children.length > MAX_ROWS) box.removeChild(box.lastChild);
    resolveProfile(ev.pubkey);
  }

  function row(ev) {
    const label = kindName(ev.kind);
    const when = relTime(ev.created_at);
    const text = ev.kind === 1 ? snippet(ev.content) : "";
    return (
      `<div class="flex items-start gap-2.5 px-3 py-2 border-b cursor-pointer border-border hover:bg-surface-elevated" data-eid="${esc(ev.id)}">` +
      `<span data-pubkey="${esc(ev.pubkey)}" class="shrink-0 cursor-pointer" title="View profile">${avatar(ev.pubkey)}</span>` +
      `<div class="flex-1 min-w-0">` +
      `<div class="flex items-center gap-1.5 text-xs text-text-secondary">` +
      `<span class="font-medium text-text">${esc(label)}</span><span>·</span><span>${esc(when)}</span></div>` +
      (text ? `<div class="mt-0.5 text-sm break-words text-text">${esc(text)}</div>` : "") +
      `</div></div>`
    );
  }

  async function resolveProfile(pubkey) {
    if (!pubkey || profiles[pubkey]) return;
    profiles[pubkey] = "pending";
    try {
      const r = await fetch("/api/v1/user/profile?pubkey=" + encodeURIComponent(pubkey));
      if (!r.ok) {
        profiles[pubkey] = {};
        return;
      }
      const ev = await r.json();
      let content = {};
      try {
        content = JSON.parse((ev && ev.content) || "{}");
      } catch (_) {}
      profiles[pubkey] = { picture: content.picture || "", name: content.display_name || content.name || "" };
      const box = document.getElementById("relay-stream-list");
      if (box) {
        box.querySelectorAll('[data-pubkey="' + cssEsc(pubkey) + '"]').forEach(function (el) {
          el.innerHTML = avatar(pubkey);
        });
      }
    } catch (_) {
      profiles[pubkey] = {};
    }
  }

  // Delegated click: avatar -> profile, rest of the row -> event page. One
  // listener, re-queried each click, so it survives SPA navigation.
  if (window.__feedClick) document.removeEventListener("click", window.__feedClick);
  window.__feedClick = function (e) {
    const root = document.getElementById("relay-stream-list");
    if (!root || !e.target.closest) return;
    const av = e.target.closest("[data-pubkey]");
    if (av && root.contains(av)) {
      e.stopPropagation();
      window.location.href = "/p/" + av.getAttribute("data-pubkey");
      return;
    }
    const r = e.target.closest("[data-eid]");
    if (r && root.contains(r)) {
      window.location.href = "/e/" + r.getAttribute("data-eid");
    }
  };
  document.addEventListener("click", window.__feedClick);

  init();
})();
