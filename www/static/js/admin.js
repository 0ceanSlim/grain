// Admin dashboard wiring. Every section is a server-rendered
// <details data-section data-method> wrapping a <form>; this file
// owns the page-level concerns that can't live in Go:
//
//   1. Per-section dirty tracking (form values diverged from
//      initial state) + a floating Save/Discard bar inside the
//      open section while dirty.
//   2. Generic submit: blobFromForm(form) -> grainNIP86.submit(
//      method, [blob]), with data-shape coercion for non-string
//      fields (booleans, numbers, list textareas).
//   3. Pending tracking (saved-to-disk but not yet applied via
//      grain_reloadconfig) + sticky footer Apply / Discard.
//
// Per-section commits add only the partial template — never JS.
// The data-shape contract:
//   - "number" → parseInt or null on empty
//   - "bool"   → checkbox.checked
//   - "lines"  → textarea.value split on \n, trimmed, empties dropped
//   - anything else / absent → raw string from FormData

(function () {
  "use strict";

  // ── Pending tracking (footer "Apply") ─────────────────────────

  const pending = new Set();

  const applyBtn = document.getElementById("admin-apply");
  const discardBtn = document.getElementById("admin-discard");
  const summaryEl = document.getElementById("admin-pending-summary");

  function refreshFooter() {
    const n = pending.size;
    summaryEl.textContent =
      n === 0
        ? "No pending changes."
        : n + " pending section" + (n === 1 ? "" : "s");
    applyBtn.disabled = n === 0;
    discardBtn.disabled = n === 0;
  }

  function setPending(sectionId, on) {
    const panel = document.querySelector('[data-section="' + sectionId + '"]');
    const badge = panel && panel.querySelector('[data-role="pending-badge"]');
    if (on) {
      pending.add(sectionId);
      if (badge) badge.classList.remove("hidden");
    } else {
      pending.delete(sectionId);
      if (badge) badge.classList.add("hidden");
    }
    refreshFooter();
  }

  // ── Toast stack ──────────────────────────────────────────────
  //
  // Stacked top-right. New toasts append into #admin-toasts so they
  // pile vertically instead of overlapping at the same position.
  // Each auto-removes after a timeout; pointer-events:none on the
  // host so toasts never block clicks underneath.

  const toastHost = document.getElementById("admin-toasts");

  function toast(msg, kind) {
    if (!toastHost) return;
    const el = document.createElement("div");
    el.textContent = msg;
    el.className =
      "px-4 py-2 rounded shadow text-sm pointer-events-auto " +
      (kind === "error"
        ? "bg-danger-dim text-danger"
        : kind === "info"
        ? "bg-info-dim text-info"
        : "bg-success-dim text-success");
    toastHost.appendChild(el);
    setTimeout(() => el.remove(), 4000);
  }

  // ── Modal (confirm + spinner states) ─────────────────────────
  //
  // Single modal host (#admin-modal) whose inner panel gets
  // swapped between states. showConfirm returns a Promise<bool>;
  // showSpinner shows an indeterminate spinner + an optional
  // countdown that ticks each second. hideModal clears the panel
  // and hides the overlay. Keeps the modal API small — every
  // future flow only needs these three primitives.

  const modalRoot = document.getElementById("admin-modal");
  const modalPanel = document.getElementById("admin-modal-panel");

  function hideModal() {
    modalRoot.classList.add("hidden");
    modalPanel.innerHTML = "";
  }

  function showConfirm(title, body) {
    return new Promise((resolve) => {
      modalPanel.innerHTML =
        '<h3 class="text-lg font-semibold text-text"></h3>' +
        '<p class="mt-2 text-sm text-text-secondary"></p>' +
        '<div class="flex justify-end gap-2 mt-5">' +
        '<button type="button" data-modal="cancel" class="px-3 py-1.5 text-sm rounded border border-border-strong text-text hover:bg-surface-hover">Cancel</button>' +
        '<button type="button" data-modal="confirm" class="px-3 py-1.5 text-sm rounded bg-accent text-accent-fg hover:bg-accent-hover">Confirm</button>' +
        "</div>";
      modalPanel.querySelector("h3").textContent = title;
      modalPanel.querySelector("p").textContent = body;
      modalRoot.classList.remove("hidden");

      const onClick = (ev) => {
        const choice = ev.target && ev.target.dataset && ev.target.dataset.modal;
        if (choice !== "cancel" && choice !== "confirm") return;
        modalPanel.removeEventListener("click", onClick);
        hideModal();
        resolve(choice === "confirm");
      };
      modalPanel.addEventListener("click", onClick);
    });
  }

  // showSpinner swaps the panel to an in-progress state. When
  // showElapsed is true, a live "Xs elapsed" line ticks each second
  // so the operator can tell the page isn't frozen. We don't
  // countdown to a target — restart time depends on DB size, GC
  // pauses, machine load, so promising "~4 s remaining" lies to
  // the operator when the actual wait runs longer.
  function showSpinner(title, body, showElapsed) {
    modalPanel.innerHTML =
      '<div class="flex items-start gap-4">' +
      '<div class="flex-shrink-0 w-8 h-8 rounded-full border-4 border-border-strong border-t-accent animate-spin"></div>' +
      '<div class="flex-1">' +
      '<h3 class="text-lg font-semibold text-text"></h3>' +
      '<p class="mt-1 text-sm text-text-secondary"></p>' +
      '<p data-role="elapsed" class="mt-2 text-xs font-mono text-text-secondary"></p>' +
      "</div></div>";
    modalPanel.querySelector("h3").textContent = title;
    modalPanel.querySelector("p").textContent = body;
    modalRoot.classList.remove("hidden");

    if (!showElapsed) return null;
    const el = modalPanel.querySelector('[data-role="elapsed"]');
    const start = Date.now();
    el.textContent = "0 s elapsed";
    const tick = setInterval(() => {
      const secs = Math.floor((Date.now() - start) / 1000);
      el.textContent = secs + " s elapsed";
    }, 1000);
    return tick;
  }

  // ── FormData → JSON blob ─────────────────────────────────────

  function coerce(field) {
    const shape = field.dataset.shape;
    if (shape === "bool") return !!field.checked;
    if (shape === "number") {
      // Empty string → null so the server treats it as "unset" rather
      // than NaN. Number() on "" is 0 which would silently zero a
      // config knob.
      const v = field.value.trim();
      if (v === "") return null;
      const n = Number(v);
      return Number.isFinite(n) ? n : null;
    }
    if (shape === "lines") {
      return field.value
        .split("\n")
        .map((s) => s.trim())
        .filter((s) => s.length > 0);
    }
    return field.value;
  }

  function blobFromForm(form) {
    const blob = {};
    const handled = new Set();
    form.querySelectorAll("[name]").forEach((el) => {
      if (el.disabled) return;
      if (handled.has(el.name)) return;
      handled.add(el.name);
      if (el.dataset.shape === "names") {
        // Multi-checkbox group sharing this name: collect every
        // CHECKED value into an array. Used by the logging
        // section's suppress_components catalog.
        blob[el.name] = Array.from(
          form.querySelectorAll('[name="' + cssEscape(el.name) + '"]:checked')
        ).map((c) => c.value);
        return;
      }
      blob[el.name] = coerce(el);
    });
    return blob;
  }

  // Fall back to a hand-rolled escape for browsers without CSS.escape
  // (older Safari on iOS). Field names in grain are all snake_case
  // ASCII so the practical risk is low, but the helper keeps the
  // querySelector call honest.
  function cssEscape(s) {
    if (window.CSS && typeof window.CSS.escape === "function") {
      return window.CSS.escape(s);
    }
    return s.replace(/[^a-zA-Z0-9_-]/g, (c) => "\\" + c);
  }

  // ── Per-section dirty state + floating save bar ──────────────

  // Map<sectionId, initialBlobJSON> — captured the first time we
  // see a section. Used to (a) detect dirty and (b) restore on Discard.
  const initial = new Map();

  function snapshotInitial(panel) {
    const id = panel.dataset.section;
    if (initial.has(id)) return;
    const form = panel.querySelector("form");
    if (!form) return;
    initial.set(id, JSON.stringify(blobFromForm(form)));
  }

  function isDirty(panel) {
    const id = panel.dataset.section;
    const form = panel.querySelector("form");
    if (!form || !initial.has(id)) return false;
    return JSON.stringify(blobFromForm(form)) !== initial.get(id);
  }

  // Build (or hide) the per-section floating save bar. Lives inside
  // the section's <details> body so it's only visible when the
  // section is open; sticky-bottom so it follows the operator as
  // they scroll through a long form (rate_limit, blacklist).
  function refreshSaveBar(panel) {
    const id = panel.dataset.section;
    const body = panel.querySelector("details > div, details > form")
      || panel.querySelector(":scope > div");
    if (!body) return;

    let bar = panel.querySelector('[data-role="save-bar"]');
    const dirty = isDirty(panel);
    const unsavedBadge = panel.querySelector('[data-role="unsaved-badge"]');

    if (!dirty) {
      if (bar) bar.remove();
      if (unsavedBadge) unsavedBadge.classList.add("hidden");
      return;
    }

    if (unsavedBadge) unsavedBadge.classList.remove("hidden");

    if (!bar) {
      bar = document.createElement("div");
      bar.dataset.role = "save-bar";
      bar.className =
        "sticky bottom-2 mt-4 -mx-4 px-4 py-2 flex items-center justify-between " +
        "rounded-lg border border-warning bg-warning-dim text-warning shadow";
      bar.innerHTML =
        '<span class="text-sm font-medium">Unsaved changes in this section</span>' +
        '<span class="space-x-2">' +
        '<button type="button" data-action="discard" class="px-3 py-1 text-sm rounded border border-warning text-warning hover:bg-warning hover:text-warning-fg">Discard</button>' +
        '<button type="button" data-action="save" class="px-3 py-1 text-sm rounded bg-accent text-accent-fg hover:bg-accent-hover">Save</button>' +
        '</span>';
      body.appendChild(bar);
    }
  }

  // Reset form fields to the snapshot. We avoid a global form.reset()
  // because that only reverts to HTML attribute defaults — for
  // dynamically-rendered server values, the snapshot blob is the
  // truth.
  function discardSection(panel) {
    const id = panel.dataset.section;
    const form = panel.querySelector("form");
    const snap = initial.get(id);
    if (!form || !snap) return;
    const blob = JSON.parse(snap);
    const handled = new Set();
    form.querySelectorAll("[name]").forEach((el) => {
      if (handled.has(el.name) && el.dataset.shape === "names") return;
      const v = blob[el.name];
      if (el.dataset.shape === "names") {
        // Multi-checkbox group: tick each checkbox whose value
        // appears in the snapshot's array, untick the rest. We
        // iterate ALL members of the group on the first hit and
        // mark the name "handled" so the outer loop doesn't redo it.
        const wanted = new Set(Array.isArray(v) ? v : []);
        form
          .querySelectorAll('[name="' + cssEscape(el.name) + '"]')
          .forEach((c) => {
            c.checked = wanted.has(c.value);
          });
        handled.add(el.name);
      } else if (el.dataset.shape === "bool") {
        el.checked = !!v;
      } else if (el.dataset.shape === "lines") {
        el.value = (Array.isArray(v) ? v : []).join("\n");
      } else if (el.dataset.shape === "number") {
        el.value = v == null ? "" : String(v);
      } else {
        el.value = v == null ? "" : String(v);
      }
    });
    refreshSaveBar(panel);
  }

  // ensureSigner makes window.grainSigner available before we try to
  // sign a NIP-86 envelope. The signer is a runtime JS object set
  // by mill-bridge's onConnected; it does NOT survive page reloads,
  // even when the server-side session cookie does. So an operator
  // who reloaded /admin (or got here from /setup) is "logged in"
  // server-side but has no signer client-side — and gets "signer
  // unavailable" on the first save. We fix it by transparently
  // re-opening mill, then polling until the bridge re-attaches the
  // signer. Same auth UX as login; just deferred to the first save.
  async function ensureSigner() {
    if (window.grainSigner && typeof window.grainSigner.signEvent === "function") {
      return;
    }
    if (typeof window.showAuthModal !== "function") {
      throw new Error("auth modal unavailable — reload the page");
    }
    toast("link your signer to save changes");
    window.showAuthModal();
    const deadline = Date.now() + 5 * 60 * 1000;
    while (Date.now() < deadline) {
      if (
        window.grainSigner &&
        typeof window.grainSigner.signEvent === "function"
      ) {
        return;
      }
      await new Promise((r) => setTimeout(r, 250));
    }
    throw new Error("signer not connected — try again");
  }

  async function saveSection(panel) {
    const id = panel.dataset.section;
    const method = panel.dataset.method;
    const form = panel.querySelector("form");
    if (!method) {
      toast("section " + id + " has no grain_* method bound", "error");
      return;
    }
    if (!form) return;

    const bar = panel.querySelector('[data-role="save-bar"]');
    const saveBtn = bar && bar.querySelector('[data-action="save"]');
    if (saveBtn) saveBtn.disabled = true;

    try {
      await ensureSigner();
      const blob = blobFromForm(form);
      await window.grainNIP86.submit(method, [blob]);
      // Re-snapshot so the form is "clean" against its new baseline.
      initial.set(id, JSON.stringify(blob));
      refreshSaveBar(panel);
      setPending(id, true);
      toast(id + " saved — apply to activate");
    } catch (err) {
      toast(err.message || String(err), "error");
    } finally {
      if (saveBtn) saveBtn.disabled = false;
    }
  }

  // ── Wire-up ─────────────────────────────────────────────────

  // Snapshot every section's initial form state on first render.
  document.querySelectorAll("[data-section]").forEach(snapshotInitial);

  // Track edits. `input` covers text/number/textarea; `change` covers
  // select + checkbox + the cases where input doesn't fire.
  ["input", "change"].forEach((evt) => {
    document.addEventListener(evt, (ev) => {
      const panel = ev.target.closest && ev.target.closest("[data-section]");
      if (panel) refreshSaveBar(panel);
    });
  });

  // Save / Discard inside a section's floating bar.
  document.addEventListener("click", (ev) => {
    const action = ev.target && ev.target.dataset && ev.target.dataset.action;
    if (action !== "save" && action !== "discard") return;
    const panel = ev.target.closest("[data-section]");
    if (!panel) return;
    if (action === "save") saveSection(panel);
    else discardSection(panel);
  });

  // Suppress the default GET-the-form behavior in case anyone presses
  // Enter inside a section <form>.
  document.addEventListener("submit", (ev) => {
    if (ev.target && ev.target.closest && ev.target.closest("[data-section]")) {
      ev.preventDefault();
      const panel = ev.target.closest("[data-section]");
      saveSection(panel);
    }
  });

  // Footer Apply: writes are already on disk; Apply just tells the
  // running config to reload via grain_reloadconfig. Section state
  // doesn't change on the dashboard — the file watcher will reload
  // server-side, and a future refinement could re-fetch each section
  // to confirm.
  applyBtn.addEventListener("click", async () => {
    // grain_reloadconfig restarts the server (~4s) and drops active
    // WebSocket clients. Confirm via an in-UI modal so an operator
    // with a chat room of users doesn't yank the rug out by accident.
    const ok = await showConfirm(
      "Apply pending changes?",
      "The relay will restart to pick up the new config. Any connected " +
        "WebSocket clients will be disconnected; well-behaved clients " +
        "reconnect on their own, but that's not guaranteed."
    );
    if (!ok) return;

    applyBtn.disabled = true;
    try {
      await ensureSigner();
      await window.grainNIP86.submit("grain_reloadconfig", []);
      // Swap to spinner + countdown state. Poll NIP-11 until the
      // new process is up, then reload the page so every form
      // re-reads its current values from the running config.
      const tick = showSpinner(
        "Restarting relay…",
        "Usually a few seconds — longer if the database is large.",
        true
      );
      try {
        await waitForRelayBack(60 * 1000);
      } finally {
        if (tick) clearInterval(tick);
      }
      window.location.reload();
    } catch (err) {
      hideModal();
      toast(err.message || String(err), "error");
      refreshFooter();
    }
  });

  // waitForRelayBack polls the root NIP-11 endpoint (which the
  // restart loop binds before anything else) until it answers,
  // capped at timeoutMs. Used after grain_reloadconfig so the
  // success toast accurately reflects "the new process is up"
  // rather than just "we sent the RPC."
  async function waitForRelayBack(timeoutMs) {
    const deadline = Date.now() + timeoutMs;
    // Brief leading pause: the response we just got beat the restart,
    // so polling immediately would just see the dying instance.
    await new Promise((r) => setTimeout(r, 1500));
    while (Date.now() < deadline) {
      try {
        const resp = await fetch("/", {
          headers: { Accept: "application/nostr+json" },
          cache: "no-store",
        });
        if (resp.ok) return;
      } catch (_) {
        // restart in progress; keep polling
      }
      await new Promise((r) => setTimeout(r, 500));
    }
    throw new Error("relay didn't come back within " + timeoutMs / 1000 + "s");
  }

  // Footer Discard: server-side state is unchanged from the operator's
  // POV — a grain_update* call already wrote to disk. "Discard" here
  // only clears the UI markers so the badges match the operator's
  // intent to walk away.
  discardBtn.addEventListener("click", () => {
    pending.forEach((id) => setPending(id, false));
    toast("pending markers cleared (already-saved changes remain on disk)");
  });

  refreshFooter();
})();
