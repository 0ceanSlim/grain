// Admin dashboard page wiring. Deliberately minimal — every section
// is a server-rendered <details> with a <form data-method="grain_*">.
// This file only handles the irreducible browser-side work:
//   1. Track which sections have been submitted but not yet applied.
//   2. Generic form submit -> grainNIP86.submit(method, [blob]).
//   3. Apply / Discard footer actions.
//
// Phase 1 ships read-only stubs in the template, so the submit path
// here is in place but exercised only by the Apply button's call to
// grain_reloadconfig. Per-section forms in Phase 2+ light up the
// rest of this flow without adding any JS.

(function () {
  "use strict";

  const pending = new Set();

  const applyBtn = document.getElementById("admin-apply");
  const discardBtn = document.getElementById("admin-discard");
  const summary = document.getElementById("admin-pending-summary");

  function refreshFooter() {
    const n = pending.size;
    if (n === 0) {
      summary.textContent = "No pending changes.";
    } else {
      summary.textContent = n + " pending section" + (n === 1 ? "" : "s");
    }
    applyBtn.disabled = n === 0;
    discardBtn.disabled = n === 0;
  }

  function markPending(sectionId) {
    pending.add(sectionId);
    const el = document.querySelector('[data-section="' + sectionId + '"]');
    if (el) {
      const badge = el.querySelector('[data-role="pending-badge"]');
      if (badge) badge.classList.remove("hidden");
    }
    refreshFooter();
  }

  function clearPending() {
    pending.forEach((id) => {
      const el = document.querySelector('[data-section="' + id + '"]');
      if (el) {
        const badge = el.querySelector('[data-role="pending-badge"]');
        if (badge) badge.classList.add("hidden");
      }
    });
    pending.clear();
    refreshFooter();
  }

  // Toast helper. Mirrors settings.js — a fixed, auto-dismissing
  // node so we don't have to wire up a real notification system for
  // a single page. Kept inline (rather than imported) so admin.js
  // has no runtime dependency on settings.js.
  function toast(msg, kind) {
    const el = document.createElement("div");
    el.textContent = msg;
    el.className =
      "fixed top-4 right-4 z-50 px-4 py-2 rounded shadow text-sm " +
      (kind === "error"
        ? "bg-danger-dim text-danger"
        : "bg-success-dim text-success");
    document.body.appendChild(el);
    setTimeout(() => el.remove(), 4000);
  }

  // Build a flat object from FormData. Phase 2+ section partials
  // will add inputs whose `name` matches the JSON field of the
  // corresponding config struct, plus optional data-shape coercion
  // for non-string fields (booleans, numbers, lists). Phase 1 has
  // no forms with inputs, so this is dormant.
  function blobFromForm(form) {
    const blob = {};
    new FormData(form).forEach((value, key) => {
      blob[key] = value;
    });
    return blob;
  }

  document.addEventListener("submit", async (ev) => {
    const form = ev.target;
    if (!(form instanceof HTMLFormElement)) return;
    const panel = form.closest("[data-section]");
    if (!panel) return;
    ev.preventDefault();

    const sectionId = panel.dataset.section;
    const method = panel.dataset.method || form.dataset.method;
    if (!method) {
      toast("section " + sectionId + " has no grain_* method bound", "error");
      return;
    }
    try {
      await window.grainNIP86.submit(method, [blobFromForm(form)]);
      markPending(sectionId);
      toast(sectionId + " saved — apply to activate");
    } catch (err) {
      toast(err.message || String(err), "error");
    }
  });

  applyBtn.addEventListener("click", async () => {
    applyBtn.disabled = true;
    try {
      await window.grainNIP86.submit("grain_reloadconfig", []);
      clearPending();
      toast("config reloaded");
    } catch (err) {
      toast(err.message || String(err), "error");
      refreshFooter();
    }
  });

  discardBtn.addEventListener("click", () => {
    // Server-side state is unchanged from the operator's POV: a
    // grain_update* call already wrote to disk and set
    // restart_pending. "Discard" here only clears the UI markers
    // so the badge state matches the user's intent to walk away.
    // A future revision could call a future grain_revertpending.
    clearPending();
    toast("pending markers cleared (already-saved changes remain on disk)");
  });

  refreshFooter();
})();
