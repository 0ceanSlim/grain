// Setup page wiring. Opens the existing mill auth modal, waits for
// the session cookie to appear (mill-bridge POSTs /api/v1/auth/login
// internally), then POSTs the resolved pubkey to /setup.
//
// Why ride mill-bridge's normal flow instead of calling MILL.open
// directly: we want the operator logged in to grain's session by
// the time we redirect to /admin, otherwise /admin would 303 them
// back. The login POST happens automatically inside mill-bridge's
// onConnected; we just wait for /api/v1/session to confirm.

(function () {
  "use strict";

  const btn = document.getElementById("setup-claim-btn");
  const successPanel = document.getElementById("setup-success");
  const conflictPanel = document.getElementById("setup-conflict");
  const conflictNpub = document.getElementById("setup-conflict-npub");
  const errorPanel = document.getElementById("setup-error");

  function hideAllPanels() {
    successPanel.classList.add("hidden");
    conflictPanel.classList.add("hidden");
    errorPanel.classList.add("hidden");
  }

  function showError(msg) {
    hideAllPanels();
    errorPanel.textContent = msg;
    errorPanel.classList.remove("hidden");
  }

  async function fetchSessionPubkey() {
    try {
      const resp = await fetch("/api/v1/session", { cache: "no-store" });
      if (!resp.ok) return null;
      const data = await resp.json();
      return data.publicKey || null;
    } catch (_) {
      return null;
    }
  }

  async function postClaim(pubkey) {
    const resp = await fetch("/setup", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pubkey: pubkey.toLowerCase() }),
    });
    if (resp.status === 200) {
      hideAllPanels();
      successPanel.classList.remove("hidden");
      setTimeout(() => {
        window.location = "/admin";
      }, 800);
      return;
    }
    if (resp.status === 409) {
      const body = await resp.json().catch(() => ({}));
      hideAllPanels();
      conflictNpub.textContent = body.owner_npub || body.owner_hex || "(unknown)";
      conflictPanel.classList.remove("hidden");
      return;
    }
    const text = await resp.text().catch(() => resp.statusText);
    showError("claim failed: " + text);
  }

  // Poll /api/v1/session for up to 60s after the operator clicked
  // claim. Mill is a modal so most of that time is the operator
  // interacting with their signer — the actual network round-trip
  // to /api/v1/auth/login is fast. We give up after 60s so a
  // closed-without-signing modal eventually re-enables the button.
  async function waitForSessionPubkey(timeoutMs) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const pk = await fetchSessionPubkey();
      if (pk) return pk;
      await new Promise((r) => setTimeout(r, 400));
    }
    return null;
  }

  btn.addEventListener("click", async () => {
    if (typeof window.showAuthModal !== "function") {
      showError("auth modal unavailable — reload the page");
      return;
    }
    hideAllPanels();
    btn.disabled = true;

    window.showAuthModal();
    try {
      const pubkey = await waitForSessionPubkey(60 * 1000);
      if (!pubkey) {
        showError("no signer connected — try again");
        return;
      }
      await postClaim(pubkey);
    } catch (err) {
      showError(err.message || String(err));
    } finally {
      btn.disabled = false;
    }
  });
})();
