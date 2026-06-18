// Reusable media upload (#83). Uploads a File to the user's chosen media server
// — Blossom (BUD-01/02, kind 24242 PUT) or NIP-96 (kind 27235 NIP-98 POST) —
// signing the auth event with the connected signer (window.grainSigner). Every
// upload pops a small pre-upload modal: pick the primary server + optionally
// mirror to the others. Returns the hosted URL to the caller.
//
// Public API:
//   window.grainUpload.pick(onUrl, opts)        — open a file picker, then upload
//   window.grainUpload.open(file, onUrl, opts)  — upload an already-chosen File
//   opts: { accept: "image/*", title: "Upload" }
(function () {
  "use strict";

  // ── crypto + auth ──────────────────────────────────────────────────────────

  async function sha256Hex(buf) {
    const digest = await crypto.subtle.digest("SHA-256", buf);
    return Array.from(new Uint8Array(digest))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  }

  async function ensureSigner() {
    if (typeof window.restoreSigner === "function") {
      try {
        await window.restoreSigner();
      } catch (e) {}
    }
    return window.grainSigner && typeof window.grainSigner.signEvent === "function"
      ? window.grainSigner
      : null;
  }

  function b64Event(evt) {
    return btoa(JSON.stringify(evt));
  }

  // ── Blossom (BUD-01/02) ────────────────────────────────────────────────────

  async function uploadBlossom(file, serverUrl, hash, onProgress) {
    const signer = await ensureSigner();
    if (!signer) throw new Error("No signer connected");
    const auth = await signer.signEvent({
      kind: 24242,
      created_at: Math.floor(Date.now() / 1000),
      content: "Upload " + file.name,
      tags: [
        ["t", "upload"],
        ["x", hash],
        ["expiration", String(Math.floor(Date.now() / 1000) + 3600)],
      ],
    });
    const base = serverUrl.replace(/\/+$/, "");
    const desc = await xhrSend("PUT", base + "/upload", file, {
      Authorization: "Nostr " + b64Event(auth),
    }, onProgress);
    const json = JSON.parse(desc);
    if (!json || !json.url) throw new Error("Server returned no blob URL");
    return json.url;
  }

  // ── NIP-96 ──────────────────────────────────────────────────────────────────

  async function uploadNIP96(file, serverUrl, hash, onProgress) {
    const signer = await ensureSigner();
    if (!signer) throw new Error("No signer connected");
    // Discover the API endpoint (BUD-style well-known).
    const base = serverUrl.replace(/\/+$/, "");
    let apiUrl = base;
    try {
      const wk = await fetch(base + "/.well-known/nostr/nip96.json");
      if (wk.ok) {
        const cfg = await wk.json();
        if (cfg && cfg.api_url) apiUrl = cfg.api_url;
      }
    } catch (e) {
      /* fall back to the base URL */
    }
    const auth = await signer.signEvent({
      kind: 27235,
      created_at: Math.floor(Date.now() / 1000),
      content: "",
      tags: [
        ["u", apiUrl],
        ["method", "POST"],
        ["payload", hash],
      ],
    });
    const form = new FormData();
    form.append("file", file);
    const respText = await xhrSend("POST", apiUrl, form, {
      Authorization: "Nostr " + b64Event(auth),
    }, onProgress);
    const json = JSON.parse(respText);
    // NIP-96 returns nip94_event with a ["url", <url>] tag.
    const tags = (json && json.nip94_event && json.nip94_event.tags) || [];
    const urlTag = tags.find((t) => t[0] === "url");
    if (!urlTag) throw new Error(json && json.message ? json.message : "Upload failed");
    return urlTag[1];
  }

  // xhrSend wraps XMLHttpRequest so we get upload progress for large files.
  function xhrSend(method, url, body, headers, onProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open(method, url, true);
      Object.keys(headers || {}).forEach((k) => xhr.setRequestHeader(k, headers[k]));
      if (xhr.upload && onProgress) {
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) onProgress(e.loaded / e.total);
        };
      }
      xhr.onload = () =>
        xhr.status >= 200 && xhr.status < 300
          ? resolve(xhr.responseText)
          : reject(new Error("HTTP " + xhr.status + (xhr.responseText ? ": " + xhr.responseText.slice(0, 200) : "")));
      xhr.onerror = () => reject(new Error("Network error (CORS or server unreachable)"));
      xhr.send(body);
    });
  }

  async function uploadTo(file, server, hash, onProgress) {
    return server.kind === "nip96"
      ? uploadNIP96(file, server.url, hash, onProgress)
      : uploadBlossom(file, server.url, hash, onProgress);
  }

  // ── Pre-upload modal ──────────────────────────────────────────────────────

  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  }
  function shortUrl(u) {
    return String(u || "").replace(/^https?:\/\//, "").replace(/\/$/, "");
  }
  function fmtSize(n) {
    return n < 1024 * 1024 ? Math.round(n / 1024) + " KB" : (n / 1024 / 1024).toFixed(1) + " MB";
  }

  // Navigate to Settings and scroll to the media-servers section (settings.js
  // reads window.__grainScrollTarget once the content is visible). Mirrors
  // header.html's openRelaySettings so it works from any page, SPA or not.
  function openMediaSettings() {
    window.__grainScrollTarget = "media-servers-section";
    if (typeof window.htmx === "undefined") {
      window.location.href = "/settings";
      return;
    }
    window.history.pushState({}, "", "/settings");
    window.htmx.ajax("GET", "/views/settings.html", { target: "#main-content" });
  }

  function showModal(file, servers, onUrl, opts, fetchOk) {
    const all = [].concat((servers && servers.blossom) || [], (servers && servers.nip96) || []);
    const hasAny = servers && servers.hasAny && all.length > 0;

    const overlay = document.createElement("div");
    overlay.className = "fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60";
    const card = document.createElement("div");
    card.className = "w-full max-w-md p-6 border shadow-lg rounded-xl bg-surface border-border";
    overlay.appendChild(card);
    let previewUrl = "";
    function close() {
      if (previewUrl) URL.revokeObjectURL(previewUrl);
      overlay.remove();
    }
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) close();
    });

    if (!hasAny) {
      const failed = !fetchOk;
      card.innerHTML =
        (failed
          ? `<h3 class="mb-2 text-lg font-semibold text-text">Couldn't load your media servers</h3>` +
            `<p class="mb-4 text-sm text-text-muted">Make sure you're signed in, then re-check.</p>`
          : `<h3 class="mb-2 text-lg font-semibold text-text">No media server set up</h3>` +
            `<p class="mb-4 text-sm text-text-muted">If you just added one it may not have synced yet — re-check, or add one in Settings → Media servers.</p>`) +
        `<div class="flex flex-wrap justify-end gap-2">` +
        `<button data-act="cancel" class="px-3 py-2 text-sm rounded-lg text-text bg-surface-elevated hover:bg-surface-hover">Close</button>` +
        `<button data-act="recheck" class="px-3 py-2 text-sm rounded-lg text-text bg-surface-elevated hover:bg-surface-hover">Re-check</button>` +
        `<button data-act="settings" class="px-3 py-2 text-sm rounded-lg text-text-on-accent bg-accent hover:opacity-80">Open settings</button>` +
        `</div>`;
      card.querySelector('[data-act="cancel"]').onclick = close;
      card.querySelector('[data-act="recheck"]').onclick = function () {
        close();
        open(file, onUrl, Object.assign({}, opts, { refresh: true }));
      };
      card.querySelector('[data-act="settings"]').onclick = function () {
        close();
        openMediaSettings();
      };
      document.body.appendChild(overlay);
      return;
    }

    // Image preview (the name + size stay below it).
    let previewHtml = "";
    if (file.type && file.type.indexOf("image/") === 0) {
      previewUrl = URL.createObjectURL(file);
      previewHtml = `<img src="${previewUrl}" alt="" class="object-contain w-full mb-3 rounded max-h-44 bg-surface-inset" />`;
    }

    const options = all
      .map((s, i) => {
        const tags = [s.kind, s.cost, s.retention].filter(Boolean).join(" · ");
        return `<option value="${i}"${s.primary ? " selected" : ""}>${esc(shortUrl(s.url))}${tags ? " — " + esc(tags) : ""}</option>`;
      })
      .join("");

    card.innerHTML =
      `<h3 class="mb-3 text-lg font-semibold text-text">Upload file</h3>` +
      previewHtml +
      `<p class="mb-3 text-xs text-text-muted">${esc(file.name)} · ${fmtSize(file.size)}</p>` +
      `<label class="block mb-1 text-xs font-medium text-text-secondary">Upload to</label>` +
      `<select data-ref="server" class="w-full px-3 py-2 mb-3 text-sm border rounded-lg bg-surface-elevated text-text border-border">${options}</select>` +
      `<label class="flex items-center gap-2 text-sm cursor-pointer text-text">` +
      `<input data-ref="mirror" type="checkbox" /> Mirror to other servers</label>` +
      `<div data-ref="mirrorList" class="hidden p-2 mt-2 space-y-1.5 border rounded-lg border-border bg-surface-inset"></div>` +
      `<p data-ref="ephemeral" class="hidden mt-2 text-xs text-warning">⚠ This server is ephemeral — uploads may be deleted after a while.</p>` +
      `<div data-ref="progress" class="hidden h-1.5 my-3 overflow-hidden rounded bg-surface-inset"><div data-ref="bar" class="h-full bg-accent" style="width:0%"></div></div>` +
      `<p data-ref="status" class="mt-3 mb-4 text-xs text-text-secondary"></p>` +
      `<div class="flex justify-end gap-2">` +
      `<button data-ref="cancel" class="px-4 py-2 text-sm rounded-lg text-text bg-surface-elevated hover:bg-surface-hover">Cancel</button>` +
      `<button data-ref="go" class="px-4 py-2 text-sm rounded-lg text-text-on-accent bg-accent hover:opacity-80">Upload</button>` +
      `</div>`;

    const $ = (ref) => card.querySelector('[data-ref="' + ref + '"]');
    const sel = $("server"),
      mirror = $("mirror"),
      mirrorList = $("mirrorList"),
      ephemeral = $("ephemeral"),
      progress = $("progress"),
      bar = $("bar"),
      status = $("status"),
      go = $("go");

    // Per-server mirror selection: list the OTHER servers (relative to the chosen
    // primary) with individual checkboxes, default all on, plus a Select all.
    function renderMirrorList() {
      const primary = Number(sel.value);
      const others = [];
      for (let i = 0; i < all.length; i++) if (i !== primary) others.push(i);
      if (!others.length) {
        mirrorList.innerHTML = `<p class="text-xs text-text-muted">No other servers to mirror to.</p>`;
        return;
      }
      mirrorList.innerHTML =
        `<label class="flex items-center gap-2 text-xs cursor-pointer text-text-secondary"><input data-ref="mirrorAll" type="checkbox" checked /> Select all</label>` +
        others
          .map(
            (i) =>
              `<label class="flex items-center gap-2 text-xs cursor-pointer text-text"><input type="checkbox" data-mirror-idx="${i}" checked /> ${esc(shortUrl(all[i].url))}</label>`
          )
          .join("");
      const allCb = mirrorList.querySelector('[data-ref="mirrorAll"]');
      allCb.onchange = () => {
        mirrorList.querySelectorAll("[data-mirror-idx]").forEach((cb) => (cb.checked = allCb.checked));
      };
    }

    function reflectServer() {
      const s = all[Number(sel.value)];
      ephemeral.classList.toggle("hidden", !(s && s.retention === "ephemeral"));
      const hasOthers = all.length > 1;
      mirror.disabled = !hasOthers;
      mirror.parentElement.classList.toggle("opacity-50", !hasOthers);
      if (mirror.checked) renderMirrorList(); // keep the list in sync with the primary
    }
    sel.onchange = reflectServer;
    mirror.onchange = () => {
      mirrorList.classList.toggle("hidden", !mirror.checked);
      if (mirror.checked) renderMirrorList();
    };
    reflectServer();
    $("cancel").onclick = close;

    go.onclick = async function () {
      const server = all[Number(sel.value)];
      const mirrorTargets =
        mirror.checked && !mirror.disabled
          ? Array.from(mirrorList.querySelectorAll("[data-mirror-idx]:checked")).map(
              (cb) => all[Number(cb.getAttribute("data-mirror-idx"))]
            )
          : [];
      go.disabled = true;
      sel.disabled = true;
      progress.classList.remove("hidden");
      status.textContent = "Hashing…";
      try {
        const buf = await file.arrayBuffer();
        const hash = await sha256Hex(buf);
        status.textContent = "Uploading to " + shortUrl(server.url) + "…";
        const url = await uploadTo(file, server, hash, (p) => {
          bar.style.width = Math.round(p * 100) + "%";
        });
        for (const t of mirrorTargets) {
          status.textContent = "Mirroring to " + shortUrl(t.url) + "…";
          try {
            await uploadTo(file, t, hash, null);
          } catch (e) {
            /* best-effort mirror */
          }
        }
        status.textContent = "✓ Uploaded";
        onUrl(url);
        setTimeout(close, 700);
      } catch (e) {
        go.disabled = false;
        sel.disabled = false;
        progress.classList.add("hidden");
        status.textContent = "Upload failed: " + (e && e.message ? e.message : e);
      }
    };

    document.body.appendChild(overlay);
  }

  // ── Public entry points ──────────────────────────────────────────────────────

  function pick(onUrl, opts) {
    opts = opts || {};
    const input = document.createElement("input");
    input.type = "file";
    input.accept = opts.accept || "image/*";
    input.style.display = "none";
    input.onchange = () => {
      if (input.files && input.files[0]) open(input.files[0], onUrl, opts);
      input.remove();
    };
    document.body.appendChild(input);
    input.click();
  }

  async function open(file, onUrl, opts) {
    opts = opts || {};
    let servers = null,
      fetchOk = false;
    try {
      const r = await fetch("/api/v1/user/media-servers" + (opts.refresh ? "?refresh=1" : ""));
      if (r.ok) {
        servers = await r.json();
        fetchOk = true;
      }
    } catch (e) {}
    showModal(file, servers, onUrl, opts, fetchOk);
  }

  window.grainUpload = { pick: pick, open: open, _uploadTo: uploadTo, _sha256Hex: sha256Hex };

  // Declarative wiring: a button with data-upload-target="<css selector>" opens
  // the picker and writes the resulting URL into the target field (firing input +
  // change so any framework/handler picks it up). data-upload-accept overrides
  // the file filter (default image/*). Delegated so it works on dynamically
  // rendered fields (profile edit form, admin sections).
  document.addEventListener("click", function (e) {
    const btn = e.target.closest && e.target.closest("[data-upload-target]");
    if (!btn) return;
    e.preventDefault();
    const target = document.querySelector(btn.getAttribute("data-upload-target"));
    if (!target) return;
    pick(
      function (url) {
        target.value = url;
        target.dispatchEvent(new Event("input", { bubbles: true }));
        target.dispatchEvent(new Event("change", { bubbles: true }));
      },
      { accept: btn.getAttribute("data-upload-accept") || "image/*" }
    );
  });
})();
