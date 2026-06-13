/**
 * Shared client-side publish helper.
 *
 * window.grainPublish.signAndPublish(unsigned, { status, onAccepted })
 *   Signs an unsigned event with the user's signer (NIP-07 / NIP-46 / Amber /
 *   key via mill), publishes it through grain's outbox-routed /api/v1/events/
 *   publish, and shows a small bottom-right toast with per-relay OK results.
 *   Returns { signed, result, accepted } on success, or null on failure (the
 *   toast surfaces the error). onAccepted(signed, result) fires once at least
 *   one relay stores the event.
 *
 * Kept generic so the profile editor, media-server settings, and the media
 * uploader can share one toast instead of each rolling their own.
 */
(function () {
  function escAttr(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  // One per-relay line in the expanded toast.
  function relayLine(r) {
    const url = r.RelayURL || r.relayURL || "";
    const sent = r.Success || r.success;
    const ok = r.Accepted || r.accepted;
    const reason = r.Reason || r.reason || "";
    const msg = r.Message || r.message || "";
    let icon, cls, note;
    if (ok) {
      icon = "✓"; cls = "text-success"; note = reason;
    } else if (!sent) {
      icon = "✗"; cls = "text-danger"; note = msg || "failed to send";
    } else if (reason) {
      icon = "✗"; cls = "text-danger"; note = reason; // relay rejected it
    } else {
      icon = "•"; cls = "text-warning"; note = "no response";
    }
    return (
      `<div class="flex items-start gap-2 text-xs">` +
      `<span class="${cls}">${icon}</span>` +
      `<span class="flex-1 min-w-0"><span class="font-mono break-all">${escAttr(url)}</span>` +
      (note ? `<span class="block text-text-secondary">${escAttr(note)}</span>` : "") +
      `</span></div>`
    );
  }

  // Small, bottom-right toast. Spinner while sending; then the accepted count.
  // Click the header to expand per-relay detail; dismiss with the ×.
  function makePublishToast(initialTitle) {
    const t = document.createElement("div");
    t.className =
      "fixed z-50 overflow-hidden text-sm border rounded-lg shadow-lg bottom-4 right-4 w-72 bg-surface border-border";
    t.innerHTML =
      `<div class="flex items-center gap-2 px-3 py-2 cursor-pointer" data-head>` +
      `<span data-icon class="inline-block w-4 h-4 border-2 rounded-full border-text-secondary border-t-transparent animate-spin"></span>` +
      `<span data-title class="flex-1 font-medium text-text">${escAttr(initialTitle || "Publishing…")}</span>` +
      `<button data-close class="px-1 text-text-secondary hover:text-text" title="Dismiss">×</button>` +
      `</div>` +
      `<div data-body class="hidden px-3 pb-2 space-y-1 overflow-y-auto border-t max-h-48 border-border"></div>`;
    document.body.appendChild(t);

    const body = t.querySelector("[data-body]");
    let timer = null;
    const dismiss = () => {
      if (timer) clearTimeout(timer);
      if (document.body.contains(t)) document.body.removeChild(t);
    };
    t.querySelector("[data-close]").onclick = (e) => {
      e.stopPropagation();
      dismiss();
    };
    t.querySelector("[data-head]").onclick = () => body.classList.toggle("hidden");

    function setIcon(text, cls) {
      const icon = t.querySelector("[data-icon]");
      icon.className = cls;
      icon.textContent = text;
    }

    return {
      status(text) {
        const el = t.querySelector("[data-title]");
        if (el) el.textContent = text;
      },
      update(result) {
        const relays = result.results || [];
        const accepted = relays.filter((r) => r.Accepted || r.accepted).length;
        setIcon(accepted > 0 ? "✓" : "•", accepted > 0 ? "text-success" : "text-warning");
        t.querySelector("[data-title]").textContent =
          `Accepted by ${accepted}/${relays.length} relays`;
        body.innerHTML =
          relays.map(relayLine).join("") ||
          '<span class="text-xs text-text-secondary">No relays targeted.</span>';
        timer = setTimeout(dismiss, 12000);
      },
      fail(msg) {
        setIcon("✗", "text-danger");
        t.querySelector("[data-title]").textContent = "Publish failed";
        body.innerHTML = `<span class="text-xs break-all text-danger">${escAttr(msg || "")}</span>`;
        body.classList.remove("hidden");
        timer = setTimeout(dismiss, 12000);
      },
      dismiss,
    };
  }

  async function signAndPublish(unsigned, opts) {
    opts = opts || {};
    const status = typeof opts.status === "function" ? opts.status : function () {};
    status("");
    // Create the toast up front so the user gets feedback even when the signer
    // step itself fails (that used to throw before any toast appeared).
    const toast = makePublishToast(opts.title);
    try {
      toast.status("Waiting for your signer…");
      if (typeof window.restoreSigner === "function") await window.restoreSigner();
      if (!window.grainSigner || typeof window.grainSigner.signEvent !== "function") {
        throw new Error("No signer available — log in with a signing method.");
      }
      const signed = await window.grainSigner.signEvent(unsigned);

      toast.status("Publishing…");
      const resp = await fetch("/api/v1/events/publish", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ event: signed }),
      });
      const result = await resp.json();
      if (!resp.ok || !result.success) {
        toast.fail(result.error || "publish failed");
        return null;
      }
      toast.update(result);
      const accepted = (result.results || []).some((r) => r.Accepted || r.accepted);
      if (accepted && typeof opts.onAccepted === "function") opts.onAccepted(signed, result);
      return { signed, result, accepted };
    } catch (err) {
      console.error("Publish failed:", err);
      toast.fail(err.message || String(err));
      return null;
    }
  }

  window.grainPublish = { signAndPublish, makePublishToast, escAttr };
})();
