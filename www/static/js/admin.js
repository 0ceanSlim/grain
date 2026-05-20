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
  // Sections need to surface validation errors too; expose the
  // toast helper so per-section inline JS can use it without
  // re-implementing the host.
  window.adminToast = toast;

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
    if (shape === "ints") {
      // Same as "lines" but coerce each entry to an integer.
      // Non-numeric entries are dropped — server would reject them
      // anyway, and silently dropping is friendlier than erroring on
      // every Save while the operator is mid-typing.
      return field.value
        .split("\n")
        .map((s) => s.trim())
        .filter((s) => s.length > 0)
        .map((s) => parseInt(s, 10))
        .filter((n) => Number.isFinite(n));
    }
    return field.value;
  }

  function blobFromForm(form) {
    const blob = {};
    const handled = new Set();
    form.querySelectorAll("[name]").forEach((el) => {
      if (el.disabled) return;
      // data-no-submit opts a named element out of the wire blob.
      // Used by sections that have radios / cosmetic inputs whose
      // values shouldn't be serialized — e.g. the mode-picker
      // radios in event_time_constraints. The visible state is
      // mirrored into a separate hidden input by the section's JS.
      if (el.dataset.noSubmit !== undefined) return;
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
      if (el.dataset.shape === "int_names") {
        // Multi-hidden-input group: every member contributes its
        // value, parsed as int. Used by list-of-kinds UIs where
        // each saved entry is a hidden input + a remove button.
        blob[el.name] = Array.from(
          form.querySelectorAll('[name="' + cssEscape(el.name) + '"]')
        )
          .map((c) => parseInt(c.value, 10))
          .filter((n) => Number.isFinite(n));
        return;
      }
      if (el.dataset.shape === "string_names") {
        // Same as int_names but values stay strings. Used by URL
        // lists (backup_relay), domain whitelists, etc.
        blob[el.name] = Array.from(
          form.querySelectorAll('[name="' + cssEscape(el.name) + '"]')
        ).map((c) => String(c.value));
        return;
      }
      if (el.dataset.shape === "json") {
        // The hidden input's value IS a JSON document. Section JS
        // owns marshalling — admin.js just round-trips. Used for
        // compound shapes (rate_limit's per-kind/per-category
        // limit lists) where every row has multiple fields and
        // the array-of-objects has no clean flat form.
        try {
          blob[el.name] = JSON.parse(el.value || "null");
        } catch (_) {
          blob[el.name] = null;
        }
        return;
      }
      if (el.dataset.shape === "map_bool") {
        // Multi-checkbox group whose value is a {key: bool} map.
        // Every member of the group contributes a key; checked
        // becomes true, unchecked stays false. Server-side an
        // explicit-false is meaningful (operators communicating
        // "don't purge this category" deliberately), so we don't
        // drop unchecked entries.
        const obj = {};
        form
          .querySelectorAll('[name="' + cssEscape(el.name) + '"]')
          .forEach((c) => {
            obj[c.value] = !!c.checked;
          });
        blob[el.name] = obj;
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
  // Section hooks let a partial's inline JS rehydrate its
  // cosmetic UI after admin.js restores hidden inputs. Section
  // registers like:
  //   window.adminSectionHooks.event_time_constraints =
  //     { rehydrate: function(panel) { ... } }
  // admin.js calls it after discardSection finishes its work.
  window.adminSectionHooks = window.adminSectionHooks || {};

  function discardSection(panel) {
    const id = panel.dataset.section;
    const form = panel.querySelector("form");
    const snap = initial.get(id);
    if (!form || !snap) return;
    const blob = JSON.parse(snap);
    const handled = new Set();
    form.querySelectorAll("[name]").forEach((el) => {
      const groupShape = el.dataset.shape;
      if (
        handled.has(el.name) &&
        (groupShape === "names" || groupShape === "map_bool")
      ) {
        return;
      }
      const v = blob[el.name];
      if (groupShape === "names") {
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
      } else if (groupShape === "map_bool") {
        // Multi-checkbox map: restore each key's bool from the
        // snapshot object. Missing keys default to false.
        const obj = v && typeof v === "object" ? v : {};
        form
          .querySelectorAll('[name="' + cssEscape(el.name) + '"]')
          .forEach((c) => {
            c.checked = !!obj[c.value];
          });
        handled.add(el.name);
      } else if (groupShape === "ints") {
        el.value = Array.isArray(v) ? v.join("\n") : "";
      } else if (groupShape === "json") {
        // Stringify so the hidden input value reflects the
        // snapshot. The section's rehydrate hook then re-parses
        // and re-renders the visible compound UI.
        el.value = v == null ? "" : JSON.stringify(v);
      } else if (groupShape === "int_names" || groupShape === "string_names") {
        // Rebuild the list widget from the snapshot. The
        // container's data-list-shape tells listRender which item
        // template to use.
        const list = form.querySelector('[data-list="' + cssEscape(el.name) + '"]');
        if (list) listRender(list, Array.isArray(v) ? v : []);
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
    // Let the section sync its cosmetic UI with the restored
    // hidden values. No-op if the section didn't register a hook.
    const hook = window.adminSectionHooks[panel.dataset.section];
    if (hook && typeof hook.rehydrate === "function") {
      hook.rehydrate(panel);
    }
    refreshSaveBar(panel);
  }

  // ensureSigner makes window.grainSigner available before we try
  // to sign a NIP-86 envelope. The signer is a runtime JS object
  // set by mill-bridge's onConnected; it does NOT survive page
  // reloads even when the server-side session cookie does.
  //
  // Strategy:
  //   1. If grainSigner already exists, return.
  //   2. Try a silent restore via mill (restoreSigner rebuilds the
  //      signer for the session's method from mill's persisted
  //      state — works for every method, not just NIP-07).
  //   3. Otherwise fall back to showing a "reconnect signer"
  //      indicator (not auto-popping mill) and rejecting the
  //      current action. Operator clicks the indicator when ready
  //      to re-link. Avoids the surprise-modal-mid-edit UX.
  async function ensureSigner() {
    if (window.grainSigner && typeof window.grainSigner.signEvent === "function") {
      return;
    }
    // Silent restore for any method (NIP-07 from window.nostr,
    // bunker/encrypted/amber from mill's persisted state).
    if (typeof window.restoreSigner === "function") {
      const ok = await window.restoreSigner();
      if (ok) return;
    }
    // Restore not possible (no persisted state, extension missing,
    // etc.): show the reconnect indicator and refuse this action.
    // The indicator wires its own click → restore/showAuthModal.
    showReconnectIndicator(true);
    throw new Error("signer not connected — click the Reconnect indicator");
  }
  window.adminEnsureSigner = ensureSigner;

  // ── Reconnect indicator ─────────────────────────────────────
  //
  // A persistent pill in the page chrome that glows when the
  // signer is missing. Click opens mill once. Replaces the old
  // auto-pop-mill-mid-edit behavior, which was surprising.
  function showReconnectIndicator(needed) {
    let pill = document.getElementById("admin-reconnect-pill");
    if (!pill) {
      pill = document.createElement("button");
      pill.id = "admin-reconnect-pill";
      pill.type = "button";
      pill.className =
        "fixed top-16 right-4 z-40 hidden px-3 py-1.5 text-xs font-medium rounded-full " +
        "bg-warning-dim text-warning border border-warning hover:bg-warning hover:text-warning-fg shadow";
      pill.textContent = "🔑 Reconnect signer";
      pill.addEventListener("click", async () => {
        // Try the silent restore first — covers the case where mill's
        // persisted state is available (or the extension just woke up
        // between page load and click). Only open the picker if that
        // fails.
        if (typeof window.restoreSigner === "function") {
          const ok = await window.restoreSigner();
          if (ok) {
            showReconnectIndicator(false);
            toast("signer reconnected");
            return;
          }
        }
        if (typeof window.showAuthModal === "function") {
          window.showAuthModal();
        }
      });
      document.body.appendChild(pill);
    }
    pill.classList.toggle("hidden", !needed);
  }

  // Hide the indicator when mill-bridge fires the signer-ready
  // event from a successful reconnect.
  window.addEventListener("grain:signer-ready", () => {
    showReconnectIndicator(false);
  });

  // expandDotted converts a flat blob with keys like
  // "pubkey_whitelist.enabled" into a nested object the server's
  // typed struct can JSON-unmarshal. Sections without dotted
  // names round-trip unchanged. Leaf values pass through as-is
  // (objects, arrays, primitives all opaque to the path walk).
  function expandDotted(blob) {
    const out = {};
    for (const key in blob) {
      if (!Object.prototype.hasOwnProperty.call(blob, key)) continue;
      const parts = key.split(".");
      if (parts.length === 1) {
        out[key] = blob[key];
        continue;
      }
      let cur = out;
      for (let i = 0; i < parts.length - 1; i++) {
        if (!cur[parts[i]] || typeof cur[parts[i]] !== "object") {
          cur[parts[i]] = {};
        }
        cur = cur[parts[i]];
      }
      cur[parts[parts.length - 1]] = blob[key];
    }
    return out;
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
      const flat = blobFromForm(form);
      // Snapshot stays flat — discard restores by walking [name]
      // elements one-by-one, which matches the flat shape. The
      // wire blob is the nested expansion.
      const wire = expandDotted(flat);
      await window.grainNIP86.submit(method, [wire]);
      initial.set(id, JSON.stringify(flat));
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

  // ── List-of-values UI (e.g. kinds_to_purge) ─────────────────
  //
  // Convention: a container with
  //   data-list="<wire-name>"       — the form field name to serialize
  //   data-list-shape="int"          — values are integers (extends later)
  // contains:
  //   <input data-list-input> + <button data-list-add>     (add row)
  //   <div data-list-items>                                (live list)
  // Each list item is a small DOM block holding a hidden input
  //   <input type="hidden" name="<wire-name>" data-shape="int_names" value="N">
  // plus a "kind — label" line and a remove button [data-list-remove].
  // blobFromForm picks up the values by name + data-shape.

  function listLabelFor(kind) {
    if (window.NOSTR_KIND_LABELS && window.NOSTR_KIND_LABELS[kind]) {
      return window.NOSTR_KIND_LABELS[kind];
    }
    return "(no description)";
  }

  // listItemHTML renders one row of a list widget. Shape decides
  // hidden-input data-shape (the wire type) plus the row's display
  // layout. All "string-wire" shapes share data-shape="string_names";
  // they differ in how they validate on Add and how the row reads
  // to the operator.
  function listItemHTML(name, value, shape) {
    const v = String(value);
    if (shape === "ws_url" || shape === "domain") {
      return (
        '<div data-list-item class="flex items-center gap-2 py-1 px-2 rounded bg-surface-elevated text-sm">' +
        '<input type="hidden" name="' + name + '" data-shape="string_names" value="' + escapeHTML(v) + '" />' +
        '<span class="font-mono text-text truncate flex-1">' + escapeHTML(v) + '</span>' +
        '<button type="button" data-list-remove class="ml-auto px-2 text-text-secondary hover:text-danger" title="Remove">✕</button>' +
        "</div>"
      );
    }
    if (shape === "hex_pubkey" || shape === "npub") {
      // Truncate long identifiers for display; full value lives
      // in the hidden input.
      const display = v.length > 24 ? v.slice(0, 12) + "…" + v.slice(-8) : v;
      return (
        '<div data-list-item class="flex items-center gap-2 py-1 px-2 rounded bg-surface-elevated text-sm">' +
        '<input type="hidden" name="' + name + '" data-shape="string_names" value="' + escapeHTML(v) + '" />' +
        '<span class="font-mono text-text truncate flex-1" title="' + escapeHTML(v) + '">' + escapeHTML(display) + '</span>' +
        '<button type="button" data-list-remove class="ml-auto px-2 text-text-secondary hover:text-danger" title="Remove">✕</button>' +
        "</div>"
      );
    }
    if (shape === "pubkey_unified") {
      // value is the hex form (canonical). The row's data-npub
      // attribute carries the bech32 form for display, and the
      // hidden input ships the hex. Add path resolves the other
      // form via /api/v1/keys/convert/public before calling this.
      const hex = v;
      const npub = (arguments.length > 3 && arguments[3]) || "";
      const shortHex = hex.length > 16 ? hex.slice(0, 10) + "…" + hex.slice(-6) : hex;
      const shortNpub = npub.length > 24 ? npub.slice(0, 12) + "…" + npub.slice(-8) : npub;
      return (
        '<div data-list-item class="flex items-center gap-2 py-1 px-2 rounded bg-surface-elevated text-sm" data-npub="' + escapeHTML(npub) + '">' +
        '<input type="hidden" name="' + name + '" data-shape="string_names" value="' + escapeHTML(hex) + '" />' +
        '<div class="flex-1 min-w-0">' +
        (npub ? '<div class="font-mono text-text text-xs truncate" title="' + escapeHTML(npub) + '">' + escapeHTML(shortNpub) + '</div>' : '') +
        '<div class="font-mono text-text-secondary text-xs truncate" title="' + escapeHTML(hex) + '">' + escapeHTML(shortHex) + '</div>' +
        '</div>' +
        '<button type="button" data-list-remove class="ml-auto px-2 text-text-secondary hover:text-danger" title="Remove">✕</button>' +
        "</div>"
      );
    }
    if (shape === "kind_str") {
      // Kind stored as string on the wire but rendered like an
      // int kind (with NIP label).
      const label = listLabelFor(parseInt(v, 10));
      return (
        '<div data-list-item class="flex items-center gap-2 py-1 px-2 rounded bg-surface-elevated text-sm">' +
        '<input type="hidden" name="' + name + '" data-shape="string_names" value="' + escapeHTML(v) + '" />' +
        '<span class="font-mono text-text">' + escapeHTML(v) + '</span>' +
        (label ? '<span class="text-text-secondary truncate">— ' + escapeHTML(label) + '</span>' : '') +
        '<button type="button" data-list-remove class="ml-auto px-2 text-text-secondary hover:text-danger" title="Remove">✕</button>' +
        "</div>"
      );
    }
    // Default: int / kind row.
    const label = listLabelFor(value);
    return (
      '<div data-list-item class="flex items-center gap-2 py-1 px-2 rounded bg-surface-elevated text-sm">' +
      '<input type="hidden" name="' + name + '" data-shape="int_names" value="' + value + '" />' +
      '<span class="font-mono text-text">' + value + '</span>' +
      '<span class="text-text-secondary truncate">— ' + escapeHTML(label) + '</span>' +
      '<button type="button" data-list-remove class="ml-auto px-2 text-text-secondary hover:text-danger" title="Remove">✕</button>' +
      "</div>"
    );
  }

  function escapeHTML(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  // listRender replaces the list-items container with rows for the
  // given values. Used by discardSection to restore snapshot state.
  function listRender(list, values) {
    const items = list.querySelector("[data-list-items]");
    if (!items) return;
    const name = list.dataset.list;
    const shape = list.dataset.listShape || "int";
    items.innerHTML = values
      .map((v) => listItemHTML(name, v, shape))
      .join("");
  }

  // listAddValue appends a value to a list widget, skipping if
  // already present. Shape-aware: parses int / validates ws_url.
  // Shared between the Add button (reads from the input) and the
  // quick-add chips (read value straight from data-quick-add).
  // Returns true on append, false otherwise (toasts on validation
  // failure so the caller doesn't have to).
  async function listAddValue(list, raw) {
    const items = list.querySelector("[data-list-items]");
    const name = list.dataset.list;
    const shape = list.dataset.listShape || "int";
    if (!items || !name) return false;

    let stored; // value as it'll be compared/serialized
    if (shape === "ws_url") {
      stored = String(raw).trim();
      if (stored === "") return false;
      if (!stored.startsWith("ws://") && !stored.startsWith("wss://")) {
        toast("url must start with ws:// or wss://", "error");
        return false;
      }
    } else if (shape === "hex_pubkey") {
      stored = String(raw).trim().toLowerCase();
      if (!/^[0-9a-f]{64}$/.test(stored)) {
        toast("pubkey must be 64-char lowercase hex", "error");
        return false;
      }
    } else if (shape === "npub") {
      stored = String(raw).trim();
      if (!/^npub1[02-9ac-hj-np-z]+$/i.test(stored) || stored.length < 60) {
        toast("must be a valid npub (bech32 string starting with npub1)", "error");
        return false;
      }
    } else if (shape === "domain") {
      stored = String(raw).trim().toLowerCase();
      // Loose check — actual DNS validation lives upstream of the
      // whitelist resolver. Catches typos like leading "@" or
      // missing dots without rejecting punycode/IDN.
      if (!stored || !stored.includes(".") || /\s/.test(stored)) {
        toast("must look like a domain (e.g. example.com)", "error");
        return false;
      }
    } else if (shape === "pubkey_unified") {
      // Accept either hex or npub on input; canonicalize to hex
      // by calling /api/v1/keys/convert/public/<input>.
      stored = String(raw).trim();
      if (stored === "") return false;
      try {
        const resp = await fetch(
          "/api/v1/keys/convert/public/" + encodeURIComponent(stored)
        );
        if (!resp.ok) throw new Error("convert " + resp.status);
        const body = await resp.json();
        // Endpoint returns {public_key} for npub-in, {npub} for hex-in.
        if (body.public_key) stored = body.public_key; // came in as npub
        const npub = body.npub || (stored === String(raw).trim() ? null : String(raw).trim());
        // Dedupe + insert with both forms.
        const items = list.querySelector("[data-list-items]");
        const name = list.dataset.list;
        const existing = items.querySelectorAll('[name="' + cssEscape(name) + '"]');
        for (let i = 0; i < existing.length; i++) {
          if (existing[i].value === stored) return false;
        }
        // Use the helper directly with the npub too.
        items.insertAdjacentHTML(
          "beforeend",
          listItemHTML(name, stored, "pubkey_unified", npub || "")
        );
        items.dispatchEvent(new Event("input", { bubbles: true }));
        return true;
      } catch (e) {
        toast("couldn't validate pubkey: " + (e.message || e), "error");
        return false;
      }
    } else if (shape === "kind_str") {
      // Wire shape is string but we validate as non-negative int.
      const n = parseInt(raw, 10);
      if (!Number.isFinite(n) || n < 0 || String(n) !== String(raw).trim()) {
        toast("kind must be a non-negative integer", "error");
        return false;
      }
      stored = String(n);
    } else {
      const n = parseInt(raw, 10);
      if (!Number.isFinite(n) || n < 0 || String(n) !== String(raw).trim()) {
        toast("must be a non-negative integer", "error");
        return false;
      }
      stored = n;
    }

    // Dedupe against existing hidden inputs.
    const existing = items.querySelectorAll(
      '[name="' + cssEscape(name) + '"]'
    );
    const stringShape = shape === "ws_url" || shape === "hex_pubkey" ||
      shape === "npub" || shape === "domain" || shape === "kind_str";
    for (let i = 0; i < existing.length; i++) {
      const eq = stringShape
        ? existing[i].value === stored
        : parseInt(existing[i].value, 10) === stored;
      if (eq) return false;
    }
    items.insertAdjacentHTML("beforeend", listItemHTML(name, stored, shape));
    items.dispatchEvent(new Event("input", { bubbles: true }));
    return true;
  }

  // Add button: read sibling input, hand the raw string to
  // listAddValue (which handles shape-specific validation —
  // including async paths that hit the convert API).
  document.addEventListener("click", async (ev) => {
    const addBtn =
      ev.target && ev.target.closest && ev.target.closest("[data-list-add]");
    if (!addBtn) return;
    const list = addBtn.closest("[data-list]");
    if (!list) return;
    const input = list.querySelector("[data-list-input]");
    if (!input) return;

    const raw = input.value.trim();
    if (raw === "") return;
    if (await listAddValue(list, raw)) input.value = "";
  });

  // Quick-add chips: trigger the same list-add path. Chip carries
  // data-quick-add (the value) and data-list-target (the list's
  // wire name). The container [data-list="<target>"] anywhere in
  // the same form is the destination.
  document.addEventListener("click", async (ev) => {
    const chip = ev.target && ev.target.closest && ev.target.closest("[data-quick-add]");
    if (!chip) return;
    const target = chip.dataset.listTarget;
    if (!target) return;
    const form = chip.closest("form");
    if (!form) return;
    const list = form.querySelector(
      '[data-list="' + cssEscape(target) + '"]'
    );
    if (!list) return;
    await listAddValue(list, chip.dataset.quickAdd);
  });

  // Remove button: drop the parent list-item.
  document.addEventListener("click", (ev) => {
    const btn =
      ev.target && ev.target.closest && ev.target.closest("[data-list-remove]");
    if (!btn) return;
    const item = btn.closest("[data-list-item]");
    if (!item) return;
    const parent = item.parentNode;
    item.remove();
    parent && parent.dispatchEvent(new Event("input", { bubbles: true }));
  });

  // Enter inside the add-input acts as Add.
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "Enter") return;
    const input = ev.target;
    if (!input || !input.matches || !input.matches("[data-list-input]")) return;
    ev.preventDefault();
    const list = input.closest("[data-list]");
    const add = list && list.querySelector("[data-list-add]");
    if (add) add.click();
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
