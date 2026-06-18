/**
 * Media servers settings section (#83).
 *
 * Renders the logged-in user's Blossom (kind 10063) and NIP-96 (kind 10096)
 * server lists, plus grain's recommended quick-adds, and lets them add / remove
 * / reorder. Save & sign rebuilds each changed list (preserving non-server tags
 * via the build endpoint), signs it with the user's signer, and publishes it
 * through the outbox. List order is meaningful: the first entry is the primary
 * upload target, the rest are mirrors.
 */
(function () {
  const MS = {
    blossom: [], // ordered base URLs the user has (your list)
    nip96: [],
    orig: { blossom: [], nip96: [] }, // as loaded — to detect changes on save
    info: {}, // url -> { cost, retention, mirror, name, note, cta, kind, known }
    suggested: { blossom: [], nip96: [] },
  };
  let msDrag = null;

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }
  function display(u) {
    return String(u || "").replace(/^https?:\/\//, "");
  }
  function cap(s) {
    return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
  }

  // Light client-side normalisation for dedupe; the server re-normalises
  // authoritatively when it builds the event.
  function norm(u) {
    let s = (u || "").trim();
    if (!s) return "";
    if (!/:\/\//.test(s)) s = "https://" + s;
    try {
      const url = new URL(s);
      if (url.protocol !== "http:" && url.protocol !== "https:") return "";
      return url.protocol + "//" + url.host.toLowerCase() + url.pathname.replace(/\/+$/, "");
    } catch (e) {
      return "";
    }
  }

  function setInfo(url, info) {
    if (url) MS.info[url] = Object.assign(MS.info[url] || {}, info);
  }
  function listFor(kind) {
    return kind === "nip96" ? MS.nip96 : MS.blossom;
  }
  function setStatus(kind, msg) {
    const el = document.getElementById("ms-" + kind + "-status");
    if (el) el.textContent = msg || "";
  }

  function chip(label, variant) {
    const styles = {
      primary: "bg-accent-dim text-accent",
      free: "bg-success-dim text-success",
      paid: "bg-warning-dim text-warning",
      permanent: "text-text-secondary border border-border-strong",
      ephemeral: "bg-warning-dim text-warning",
    };
    const cls = styles[variant] || "text-text-secondary border border-border-strong";
    return `<span class="inline-flex items-center px-2 py-0.5 text-xs font-medium rounded ${cls}">${esc(label)}</span>`;
  }

  function chipsFor(url, isPrimary) {
    const info = MS.info[url] || {};
    const out = [];
    if (isPrimary) out.push(chip("Primary", "primary"));
    if (info.cost) out.push(chip(cap(info.cost), info.cost));
    if (info.retention) out.push(chip(cap(info.retention), info.retention));
    return out.join("");
  }

  // The small info line under the chips (capability note + optional CTA link).
  function noteFor(url) {
    const info = MS.info[url] || {};
    if (!info.note) return "";
    const cta = info.cta
      ? ` <a href="${esc(info.cta)}" target="_blank" rel="noopener" class="text-accent hover:underline">membership ↗</a>`
      : "";
    return `<div class="mt-1 text-xs leading-snug text-text-muted">${esc(info.note)}${cta}</div>`;
  }

  function yourRow(kind, url, idx) {
    return (
      `<div class="flex items-center gap-2.5 px-3 py-2 border rounded-lg bg-surface-elevated border-border" data-idx="${idx}">` +
      `<span class="text-lg cursor-grab select-none shrink-0 text-text-muted" draggable="true" data-grip title="Drag to reorder">⠿</span>` +
      `<div class="flex-1 min-w-0">` +
      `<a href="${esc(url)}" target="_blank" rel="noopener" class="block text-sm font-medium truncate text-text hover:text-accent hover:underline">${esc(display(url))}</a>` +
      `<div class="flex flex-wrap items-center gap-1 mt-1">${chipsFor(url, idx === 0)}</div>` +
      noteFor(url) +
      `</div>` +
      `<button data-remove class="px-1 text-lg leading-none shrink-0 text-text-muted hover:text-danger" title="Remove">×</button>` +
      `</div>`
    );
  }

  function recRow(kind, info) {
    const chips = [];
    if (info.cost) chips.push(chip(cap(info.cost), info.cost));
    if (info.retention) chips.push(chip(cap(info.retention), info.retention));
    const cta = info.cta
      ? ` <a href="${esc(info.cta)}" target="_blank" rel="noopener" class="text-accent hover:underline">membership ↗</a>`
      : "";
    return (
      `<div class="flex items-center gap-2.5 px-3 py-2 border rounded-lg bg-surface-elevated border-border">` +
      `<div class="flex-1 min-w-0">` +
      `<a href="${esc(info.url)}" target="_blank" rel="noopener" class="block text-sm font-medium truncate text-text hover:text-accent hover:underline">${esc(display(info.url))}</a>` +
      `<div class="flex flex-wrap items-center gap-1 mt-1">${chips.join("")}</div>` +
      `<div class="mt-1 text-xs leading-snug text-text-muted">${esc(info.note || "")}${cta}</div>` +
      `</div>` +
      `<button data-add data-kind="${esc(kind)}" data-url="${esc(info.url)}" class="px-3 py-1 text-xs rounded-lg shrink-0 whitespace-nowrap text-text bg-surface-overlay hover:bg-surface-hover">+ Add</button>` +
      `</div>`
    );
  }

  function emptyState(kind) {
    const msg =
      kind === "nip96"
        ? "No NIP-96 servers. Prefer Blossom above — add one here only if you need an HTTP host."
        : "No Blossom servers yet. Add one from the recommended list below, or paste a URL.";
    return `<div class="p-4 text-sm text-center border border-dashed rounded-lg text-text-muted border-border-strong">${msg}</div>`;
  }

  function renderList(kind) {
    const box = document.getElementById("ms-" + kind + "-list");
    if (!box) return;
    const list = listFor(kind);
    if (list.length === 0) {
      box.innerHTML = emptyState(kind);
      return;
    }
    box.innerHTML = list.map((url, i) => yourRow(kind, url, i)).join("");
    box.querySelectorAll("[data-grip]").forEach((g) => {
      g.ondragstart = (e) => {
        msDrag = { kind: kind, from: +g.closest("[data-idx]").dataset.idx };
        if (e.dataTransfer) e.dataTransfer.setData("text/plain", "");
      };
    });
    box.querySelectorAll("[data-idx]").forEach((row) => {
      row.ondragover = (e) => e.preventDefault();
      row.ondrop = (e) => {
        e.preventDefault();
        if (msDrag && msDrag.kind === kind) moveServer(kind, msDrag.from, +row.dataset.idx);
        msDrag = null;
      };
    });
    box.querySelectorAll("[data-remove]").forEach((b) => {
      b.onclick = () => {
        list.splice(+b.closest("[data-idx]").dataset.idx, 1);
        renderAll();
      };
    });
  }

  function renderSuggested(kind) {
    const box = document.getElementById("ms-" + kind + "-suggested");
    if (!box) return;
    const have = new Set(listFor(kind));
    const recs = (kind === "nip96" ? MS.suggested.nip96 : MS.suggested.blossom).filter(
      (info) => !have.has(info.url)
    );
    box.innerHTML =
      recs.map((info) => recRow(kind, info)).join("") ||
      `<p class="text-xs text-text-muted">All recommended servers added.</p>`;
    box.querySelectorAll("[data-add]").forEach((b) => {
      b.onclick = () => addServer(b.dataset.kind, b.dataset.url);
    });
  }

  function renderAll() {
    renderList("blossom");
    renderSuggested("blossom");
    renderList("nip96");
    renderSuggested("nip96");
  }

  function addServer(kind, url) {
    const u = norm(url);
    if (!u) return;
    const list = listFor(kind);
    if (list.indexOf(u) >= 0) return; // already present
    list.push(u);
    renderAll();
  }

  function moveServer(kind, from, to) {
    if (from == null || to == null || from === to) return;
    const list = listFor(kind);
    const [item] = list.splice(from, 1);
    list.splice(to, 0, item);
    renderAll();
  }

  function sameList(a, b) {
    if (a.length !== b.length) return false;
    return a.every((v, i) => v === b[i]);
  }

  async function msSave(kind) {
    if (!window.grainPublish) {
      setStatus(kind, "Publish helper not loaded — reload the page.");
      return;
    }
    const numericKind = kind === "nip96" ? 10096 : 10063;
    const label = kind === "nip96" ? "NIP-96" : "Blossom";
    const list = listFor(kind);
    const orig = kind === "nip96" ? MS.orig.nip96 : MS.orig.blossom;
    if (sameList(list, orig)) {
      setStatus(kind, "No changes to save.");
      return;
    }

    const btn = document.getElementById("ms-" + kind + "-save");
    if (btn) btn.disabled = true;
    try {
      setStatus(kind, "Building your " + label + " list…");
      const resp = await fetch("/api/v1/user/media-servers/build", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          kind: numericKind,
          servers: list,
          client_tag: window.grainClientTag ? window.grainClientTag.enabled() : true,
        }),
      });
      if (!resp.ok) {
        setStatus(kind, "Couldn't build the list: " + (await resp.text()));
        return;
      }
      const unsigned = await resp.json();
      const res = await window.grainPublish.signAndPublish(unsigned, {
        title: "Publishing " + label + " list…",
        status: function (m) {
          setStatus(kind, m);
        },
      });
      if (res && res.accepted) {
        if (kind === "nip96") MS.orig.nip96 = MS.nip96.slice();
        else MS.orig.blossom = MS.blossom.slice();
        setStatus(kind, "✓ Saved — published to your relays.");
      } else if (res) {
        // Signed + sent, but no relay stored it (e.g. nowhere good to route a
        // 10063/10096 yet). The toast lists the per-relay reasons.
        setStatus(kind, "Signed, but no relay accepted it yet — see the toast.");
      } else {
        setStatus(kind, "Publish failed — see the toast or the browser console.");
      }
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  // fetch with a hard timeout so a stalled connection can't leave the section
  // spinning forever — on failure we render a retry instead of an endless spinner.
  function fetchTimeout(url, ms) {
    const ctrl = new AbortController();
    const id = setTimeout(() => ctrl.abort(), ms || 15000);
    return fetch(url, { signal: ctrl.signal }).finally(() => clearTimeout(id));
  }

  function renderListError(msg) {
    ["ms-blossom-list", "ms-nip96-list"].forEach((id) => {
      const el = document.getElementById(id);
      if (!el) return;
      el.innerHTML =
        '<div class="flex items-center justify-between gap-2 p-3 text-sm border border-dashed rounded-lg text-text-muted border-border-strong">' +
        "<span>" + esc(msg) + "</span>" +
        '<button data-ms-retry class="px-2 py-1 text-xs rounded shrink-0 text-text bg-surface-overlay hover:bg-surface-hover">Retry</button>' +
        "</div>";
    });
    document.querySelectorAll("[data-ms-retry]").forEach((b) => {
      b.onclick = initMediaServers;
    });
  }

  async function initMediaServers() {
    const sec = document.getElementById("media-servers-section");
    if (!sec) return;
    // Loading state while the lists resolve from the relays (a few seconds cold).
    ["ms-blossom-list", "ms-nip96-list"].forEach((id) => {
      const el = document.getElementById(id);
      if (el)
        el.innerHTML =
          '<div class="flex items-center gap-2 px-3 py-3 text-sm text-text-muted">' +
          '<span class="inline-block w-4 h-4 border-2 rounded-full border-text-secondary border-t-transparent animate-spin"></span>' +
          "Fetching your media servers…</div>";
    });
    try {
      const [meRes, sugRes] = await Promise.all([
        fetchTimeout("/api/v1/user/media-servers", 15000),
        fetchTimeout("/api/v1/media-servers/suggested", 15000),
      ]);
      if (meRes.ok) {
        const me = await meRes.json();
        MS.blossom = (me.blossom || []).map((e) => {
          setInfo(e.url, e);
          return e.url;
        });
        MS.nip96 = (me.nip96 || []).map((e) => {
          setInfo(e.url, e);
          return e.url;
        });
      }
      if (sugRes.ok) {
        const sug = await sugRes.json();
        (sug || []).forEach((info) => {
          setInfo(info.url, info);
          (info.kind === "nip96" ? MS.suggested.nip96 : MS.suggested.blossom).push(info);
        });
      }
      MS.orig.blossom = MS.blossom.slice();
      MS.orig.nip96 = MS.nip96.slice();
      renderAll();
    } catch (e) {
      console.error("Media servers init failed:", e);
      renderListError(
        e && e.name === "AbortError"
          ? "Timed out loading your media servers."
          : "Couldn't load your media servers."
      );
    }
  }

  // Public handlers (referenced from settings.html).
  window.initMediaServers = initMediaServers;
  window.msSave = msSave;
  window.msAddBlossomFromInput = function () {
    const i = document.getElementById("ms-blossom-input");
    if (i && i.value.trim()) {
      addServer("blossom", i.value);
      i.value = "";
    }
  };
  window.msAddNip96FromInput = function () {
    const i = document.getElementById("ms-nip96-input");
    if (i && i.value.trim()) {
      addServer("nip96", i.value);
      i.value = "";
    }
  };

  // Live-sync (#87): re-pull a media list when it changes here or in another
  // client. Re-wire on each load (the handler closes over this module's state).
  if (window.__msStreamHandler)
    window.removeEventListener("grain:stream", window.__msStreamHandler);
  window.__msStreamHandler = function (e) {
    const m = e.detail || {};
    if (m.type !== "list-updated" || (m.kind !== 10063 && m.kind !== 10096)) return;
    fetch("/api/v1/user/media-servers")
      .then((r) => (r.ok ? r.json() : null))
      .then((me) => {
        if (!me) return;
        if (m.kind === 10063) {
          MS.blossom = (me.blossom || []).map((x) => {
            setInfo(x.url, x);
            return x.url;
          });
          MS.orig.blossom = MS.blossom.slice();
          renderList("blossom");
          renderSuggested("blossom");
        } else {
          MS.nip96 = (me.nip96 || []).map((x) => {
            setInfo(x.url, x);
            return x.url;
          });
          MS.orig.nip96 = MS.nip96.slice();
          renderList("nip96");
          renderSuggested("nip96");
        }
      })
      .catch(function () {});
  };
  window.addEventListener("grain:stream", window.__msStreamHandler);

  // The script ships inside the HTMX-swapped settings view, so the section is
  // already in the DOM when this runs.
  if (document.getElementById("media-servers-section")) initMediaServers();
})();
