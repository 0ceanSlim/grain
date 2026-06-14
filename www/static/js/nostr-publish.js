/**
 * Shared client-side publish helper with a live broadcast toast.
 *
 * window.grainPublish.signAndPublish(unsigned, { title, onAccepted })
 *   Signs an unsigned event with the user's signer (NIP-07 / NIP-46 / Amber /
 *   key via mill), POSTs it to the streaming publish endpoint, and drives a
 *   small bottom-right toast that counts up live as each relay answers — a
 *   spinner + x/N pill, expandable to the per-relay accept / reject / no-response
 *   detail. The toast persists until dismissed (it only auto-fades on a fully
 *   clean success). Returns { signed, results, accepted } or null on failure.
 *
 * Shared so the profile editor, media-server settings, the relay manager, and
 * the media uploader all get the same toast.
 */
(function () {
  function escAttr(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }
  function shortRelay(u) {
    return String(u || "").replace(/^wss?:\/\//, "").replace(/\/$/, "");
  }

  // Singleton bottom-right stack so multiple publishes (e.g. Blossom then
  // NIP-96) sit above each other instead of overlapping.
  function toastStack() {
    let el = document.getElementById("grain-toast-stack");
    if (!el) {
      el = document.createElement("div");
      el.id = "grain-toast-stack";
      el.style.cssText =
        "position:fixed;right:1rem;bottom:1rem;z-index:2147483000;display:flex;" +
        "flex-direction:column;gap:0.5rem;align-items:flex-end;pointer-events:none;";
      document.body.appendChild(el);
    }
    return el;
  }

  // Maps a per-relay result to its icon/colour/note.
  function statusBits(r) {
    if (r.accepted) return { icon: "✓", cls: "text-success", note: r.reason || "" };
    if (r.sent === false) return { icon: "✗", cls: "text-danger", note: r.message || "failed to send" };
    if (r.reason) return { icon: "✗", cls: "text-danger", note: r.reason };
    if (r.pending) return { icon: "•", cls: "text-text-muted", note: "waiting…" };
    return { icon: "•", cls: "text-warning", note: "no response" };
  }

  function makePublishToast(initialTitle) {
    const t = document.createElement("div");
    // Inline layout so the toast is visible regardless of utility-class
    // availability or any stacking context on the host page.
    t.style.cssText =
      "pointer-events:auto;width:18rem;max-width:90vw;overflow:hidden;" +
      "background:var(--color-surface);border:1px solid var(--color-border);" +
      "border-radius:0.75rem;box-shadow:var(--shadow-lg);font-size:0.875rem;";
    t.innerHTML =
      `<div data-head style="display:flex;align-items:center;gap:0.5rem;padding:0.5rem 0.75rem;cursor:pointer;">` +
      `<span data-icon class="inline-block w-4 h-4 border-2 rounded-full border-text-secondary border-t-transparent animate-spin" style="flex:none;"></span>` +
      `<span data-title class="text-text" style="flex:1;min-width:0;font-weight:500;">${escAttr(initialTitle || "Broadcasting…")}</span>` +
      `<button data-toggle title="Details" class="text-text-secondary hover:text-text" style="flex:none;padding:0 0.25rem;">▾</button>` +
      `<button data-close title="Dismiss" class="text-text-secondary hover:text-text" style="flex:none;padding:0 0.25rem;">×</button>` +
      `</div>` +
      `<div data-body style="display:none;border-top:1px solid var(--color-border);max-height:12rem;overflow-y:auto;padding:0.25rem 0;"></div>`;
    toastStack().appendChild(t);

    const titleEl = t.querySelector("[data-title]");
    const iconEl = t.querySelector("[data-icon]");
    const toggleEl = t.querySelector("[data-toggle]");
    const body = t.querySelector("[data-body]");
    const rows = {};
    let total = 0;
    let accepted = 0;
    let timer = null;
    let autoMs = 0;
    let expanded = false;

    function dismiss() {
      if (timer) clearTimeout(timer);
      if (t.parentNode) t.parentNode.removeChild(t);
    }
    // Auto-dismiss after a delay, paused while the pointer is over the toast so
    // the per-relay detail stays readable. Re-armed on mouse-leave.
    function scheduleDismiss(ms) {
      autoMs = ms;
      if (timer) clearTimeout(timer);
      if (ms > 0) timer = setTimeout(dismiss, ms);
    }
    t.addEventListener("mouseenter", function () {
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
    });
    t.addEventListener("mouseleave", function () {
      if (autoMs > 0 && t.parentNode) scheduleDismiss(autoMs);
    });
    function setExpanded(on) {
      expanded = on;
      body.style.display = on ? "block" : "none";
      toggleEl.textContent = on ? "▴" : "▾";
    }
    t.querySelector("[data-close]").onclick = (e) => { e.stopPropagation(); dismiss(); };
    toggleEl.onclick = (e) => { e.stopPropagation(); setExpanded(!expanded); };
    t.querySelector("[data-head]").onclick = () => setExpanded(!expanded);

    function setIcon(symbol, cls) {
      iconEl.className = cls;
      iconEl.style.cssText = "flex:none;width:1rem;text-align:center;font-weight:700;";
      iconEl.textContent = symbol;
    }
    function rowFor(url) {
      if (rows[url]) return rows[url];
      const row = document.createElement("div");
      row.style.cssText = "display:flex;align-items:flex-start;gap:0.5rem;padding:0.2rem 0.75rem;font-size:0.75rem;";
      row.innerHTML =
        `<span data-ri style="flex:none;width:0.9rem;text-align:center;"></span>` +
        `<span style="flex:1;min-width:0;"><span data-ru class="font-mono break-all"></span>` +
        `<span data-rn class="block text-text-secondary"></span></span>`;
      row.querySelector("[data-ru]").textContent = shortRelay(url);
      body.appendChild(row);
      rows[url] = row;
      return row;
    }
    function paint(url, bits) {
      const row = rowFor(url);
      const ri = row.querySelector("[data-ri]");
      ri.className = bits.cls;
      ri.textContent = bits.icon;
      const rn = row.querySelector("[data-rn]");
      rn.textContent = bits.note || "";
      rn.style.display = bits.note ? "block" : "none";
    }

    return {
      status(text) {
        titleEl.textContent = text;
      },
      start(relays) {
        relays = relays || [];
        total = relays.length;
        relays.forEach((u) => paint(u, statusBits({ pending: true })));
        titleEl.textContent = total ? `Broadcasting… 0/${total}` : "Broadcasting…";
      },
      addResult(msg) {
        if (!msg.relayURL) return;
        paint(msg.relayURL, statusBits(msg));
        if (msg.accepted) accepted++;
        titleEl.textContent = `Broadcasting… ${accepted}/${total}`;
      },
      done() {
        if (accepted > 0) {
          setIcon("✓", "text-success");
          titleEl.textContent = `Sent to ${accepted}/${total} relays`;
        } else {
          setIcon("•", "text-warning");
          titleEl.textContent = `No relay accepted (0/${total})`;
        }
        // A fully clean success fades quickly; a partial / empty result lingers
        // longer (so the per-relay detail can be read) then clears itself.
        scheduleDismiss(total > 0 && accepted === total ? 15000 : 30000);
      },
      fail(message) {
        setIcon("✗", "text-danger");
        titleEl.textContent = "Publish failed";
        body.innerHTML =
          `<div class="text-danger break-all" style="padding:0.25rem 0.75rem;font-size:0.75rem;">${escAttr(message || "")}</div>`;
        setExpanded(true);
        scheduleDismiss(45000);
      },
      dismiss,
    };
  }

  async function signAndPublish(unsigned, opts) {
    opts = opts || {};
    const toast = makePublishToast(opts.title);
    try {
      toast.status("Waiting for your signer…");
      if (typeof window.restoreSigner === "function") await window.restoreSigner();
      if (!window.grainSigner || typeof window.grainSigner.signEvent !== "function") {
        throw new Error("No signer available — log in with a signing method.");
      }
      const signed = await window.grainSigner.signEvent(unsigned);

      toast.status("Broadcasting…");
      const resp = await fetch("/api/v1/events/publish/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ event: signed }),
      });
      if (!resp.ok) {
        const txt = await resp.text().catch(() => "");
        toast.fail(txt || "HTTP " + resp.status);
        return null;
      }
      if (!resp.body) {
        toast.fail("No response stream.");
        return null;
      }

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      const results = [];
      let acceptedAny = false;
      let buf = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        let nl;
        while ((nl = buf.indexOf("\n")) >= 0) {
          const line = buf.slice(0, nl).trim();
          buf = buf.slice(nl + 1);
          if (!line) continue;
          let m;
          try {
            m = JSON.parse(line);
          } catch (e) {
            continue;
          }
          if (m.type === "start") toast.start(m.relays || []);
          else if (m.type === "result") {
            results.push(m);
            if (m.accepted) acceptedAny = true;
            toast.addResult(m);
          } else if (m.type === "done") toast.done();
        }
      }

      if (acceptedAny && typeof opts.onAccepted === "function") opts.onAccepted(signed, results);
      return { signed, results, accepted: acceptedAny };
    } catch (err) {
      console.error("Publish failed:", err);
      toast.fail(err.message || String(err));
      return null;
    }
  }

  window.grainPublish = { signAndPublish, makePublishToast, escAttr };
})();
