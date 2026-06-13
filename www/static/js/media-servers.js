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
  function setStatus(msg) {
    const el = document.getElementById("ms-status");
    if (el) el.textContent = msg || "";
  }

  function chip(label, variant) {
    const styles = {
      primary: "bg-accent-dim text-accent",
      free: "bg-success-dim text-success",
      paid: "bg-warning-dim text-warning",
      permanent: "bg-surface-inset text-text-secondary",
      ephemeral: "bg-warning-dim text-warning",
    };
    const cls = styles[variant] || "bg-surface-inset text-text-secondary";
    return `<span class="px-2 py-0.5 text-xs rounded ${cls}">${esc(label)}</span>`;
  }

  function chipsFor(url, isPrimary) {
    const info = MS.info[url] || {};
    const out = [];
    if (isPrimary) out.push(chip("Primary", "primary"));
    if (info.cost) out.push(chip(cap(info.cost), info.cost));
    if (info.retention) out.push(chip(cap(info.retention), info.retention));
    return out.join("");
  }

  function yourRow(kind, url, idx) {
    return (
      `<div class="flex items-center gap-3 p-2.5 border rounded-lg bg-surface-elevated border-border" data-idx="${idx}">` +
      `<span class="cursor-grab select-none text-text-muted" draggable="true" data-grip title="Drag to reorder">⠿</span>` +
      `<div class="flex-1 min-w-0">` +
      `<div class="text-sm font-medium truncate text-text">${esc(display(url))}</div>` +
      `<div class="flex flex-wrap gap-1.5 mt-1">${chipsFor(url, idx === 0)}</div>` +
      `</div>` +
      `<button data-remove class="px-1 text-lg leading-none text-text-muted hover:text-danger" title="Remove">×</button>` +
      `</div>`
    );
  }

  function recRow(kind, info) {
    const chips = [];
    if (info.cost) chips.push(chip(cap(info.cost), info.cost));
    if (info.retention) chips.push(chip(cap(info.retention), info.retention));
    const cta = info.cta
      ? ` <a href="${esc(info.cta)}" target="_blank" rel="noopener" class="text-accent">membership ↗</a>`
      : "";
    return (
      `<div class="flex items-center gap-3 p-2.5 border rounded-lg bg-surface-elevated border-border">` +
      `<div class="flex-1 min-w-0">` +
      `<div class="text-sm font-medium truncate text-text">${esc(display(info.url))}</div>` +
      `<div class="flex flex-wrap gap-1.5 mt-1">${chips.join("")}</div>` +
      `<div class="mt-1 text-xs text-text-muted">${esc(info.note || "")}${cta}</div>` +
      `</div>` +
      `<button data-add data-kind="${esc(kind)}" data-url="${esc(info.url)}" class="px-3 py-1 text-xs rounded-lg whitespace-nowrap text-text bg-surface-overlay hover:bg-surface-hover">+ Add</button>` +
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

  async function msSave() {
    if (!window.grainPublish) {
      setStatus("Publish helper not loaded — reload the page.");
      return;
    }
    const changed = [];
    if (!sameList(MS.blossom, MS.orig.blossom)) changed.push({ kind: 10063, servers: MS.blossom });
    if (!sameList(MS.nip96, MS.orig.nip96)) changed.push({ kind: 10096, servers: MS.nip96 });
    if (changed.length === 0) {
      setStatus("No changes to save.");
      return;
    }

    const btn = document.getElementById("ms-save-btn");
    if (btn) btn.disabled = true;
    try {
      for (const c of changed) {
        const label = c.kind === 10063 ? "Blossom" : "NIP-96";
        setStatus("Building your " + label + " list…");
        const resp = await fetch("/api/v1/user/media-servers/build", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(c),
        });
        if (!resp.ok) {
          setStatus("Couldn't build the " + label + " list: " + (await resp.text()));
          return;
        }
        const unsigned = await resp.json();
        const res = await window.grainPublish.signAndPublish(unsigned, {
          title: "Publishing " + label + " list…",
          status: setStatus,
        });
        if (res && res.accepted) {
          if (c.kind === 10063) MS.orig.blossom = MS.blossom.slice();
          else MS.orig.nip96 = MS.nip96.slice();
        } else {
          return; // toast already explained the failure; stop before the next list
        }
      }
      setStatus("Saved.");
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  async function initMediaServers() {
    const sec = document.getElementById("media-servers-section");
    if (!sec) return;
    try {
      const [meRes, sugRes] = await Promise.all([
        fetch("/api/v1/user/media-servers"),
        fetch("/api/v1/media-servers/suggested"),
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
      setStatus("Couldn't load your media servers.");
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

  // The script ships inside the HTMX-swapped settings view, so the section is
  // already in the DOM when this runs.
  if (document.getElementById("media-servers-section")) initMediaServers();
})();
