// GRAIN → mill bridge.
//
// Mill (window.MILL, loaded by /static/mill/mill.umd.min.js) owns
// the login modal UI and produces a signer object covering every
// supported method (NIP-07, NIP-46, NIP-55, private key, read-only,
// generate). This bridge:
//
//   1. Replaces the old window.showAuthModal() / hideAuthModal()
//      surface so the login button keeps working without template
//      changes.
//   2. On mill:connected, maps mill's method id to grain's
//      SigningMethod enum (defined in client/session/types.go) and
//      POSTs /api/v1/auth/login to mint the server session cookie.
//   3. Stashes result.signer on window.grainSigner so post-login
//      callers (event publish, future NIP-86 admin POSTs) can sign
//      via .signEvent(event) without caring which method backed it.
//
// Mill auto-creates the <nostr-signer> element under document.body
// the first time MILL.open() runs. Grain's CSS bridge in input.css
// applies grain's design tokens to it via the universal selector
// `nostr-signer { --mill-*: var(--color-*) }`, so no JS theme
// handoff is needed.

(function () {
  "use strict";

  // Map mill's method id (the `method` field in mill:connected's
  // detail) to grain's server-side SigningMethod enum. Kept in sync
  // with client/session/types.go:SigningMethod.
  const METHOD_MAP = {
    nip07: "browser_extension",
    nip46: "bunker",
    nip55: "amber",
    privatekey: "encrypted_key",
    newkey: "encrypted_key",
    readonly: "none",
  };

  // The login-button template invokes `showAuthModal()` inline on
  // click. Keep that surface so we don't have to edit templates;
  // route it through mill.
  // The app name shown to the user's remote signer / bunker when authorizing
  // comes from this relay's NIP-11 `name` field, so each deployment identifies
  // itself (e.g. "🌾 GRAIN Relay") rather than a generic label. Cached after
  // the first fetch; pre-warmed on load so it's ready by the time the operator
  // clicks login.
  let relayNameCache = null;
  async function getRelayName() {
    if (relayNameCache !== null) return relayNameCache;
    try {
      const r = await fetch("/", { headers: { Accept: "application/nostr+json" } });
      const info = r.ok ? await r.json() : null;
      relayNameCache = (info && info.name) || "";
    } catch (_) {
      relayNameCache = "";
    }
    return relayNameCache;
  }
  getRelayName(); // pre-warm

  async function showAuthModal() {
    if (!window.MILL) {
      console.error(
        "[mill-bridge] MILL global not loaded — check /static/mill/mill.umd.min.js"
      );
      return;
    }
    const appName = (await getRelayName()) || document.title || "grain";
    window.MILL.open({
      // Initial paint uses mill's grain theme; the CSS bridge takes
      // over once the element is in the DOM and renders.
      theme: "grain",
      // Name the remote signer / bunker shows when authorizing (mill >= 1.2.0).
      appName,
      amberCallback:
        window.location.origin + "/api/v1/auth/amber-callback",
      onConnected: handleConnected,
    });
  }

  function hideAuthModal() {
    window.MILL?.close();
  }

  async function handleConnected(result) {
    // result: { method, pubkey, signer, perms?, bunkerUrl?, nsec? }
    window.grainSigner = result.signer || null;
    window.grainSignerMethod = result.method;
    // Tell listeners (admin dashboard reconnect indicator, etc.)
    // the signer is back. Fires for fresh logins AND on-demand
    // mill reconnects.
    if (window.grainSigner) {
      window.dispatchEvent(new CustomEvent("grain:signer-ready"));
    }

    const signingMethod = METHOD_MAP[result.method] ?? "none";
    const requestedMode = result.method === "readonly" ? "read_only" : "write";

    // Close mill immediately and flip the header button into a
    // spinner state. /api/v1/auth/login currently synchronously
    // fetches the user's mailboxes + metadata from outbox relays
    // (slow — see the v0.8 outbox-model issue), so the gap between
    // "mill closed" and "pfp+name rendered" can be several seconds.
    // Without this visual the page looks frozen.
    window.MILL?.close();
    if (typeof window.renderLoginLoading === "function") {
      window.renderLoginLoading();
    }

    try {
      const resp = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          public_key: result.pubkey,
          requested_mode: requestedMode,
          signing_method: signingMethod,
        }),
      });

      if (!resp.ok) {
        const body = await resp.text();
        console.error("[mill-bridge] /api/v1/auth/login failed:", resp.status, body);
        return;
      }

      // Swap login button for the user dropdown. We call the
      // navigation helper directly rather than dispatching the
      // "updateNav" CustomEvent — the listener binds to document.body
      // and an event fired on window doesn't bubble down to it.
      // forceNavigationUpdate does a fresh, cache-busted
      // /api/v1/session check; same path the logout flow uses, and
      // it'll replace the spinner state with pfp + display name.
      if (typeof window.forceNavigationUpdate === "function") {
        window.forceNavigationUpdate();
      }
    } catch (err) {
      console.error("[mill-bridge] login request errored:", err);
      // Roll back to the logged-out look so the user can retry.
      if (typeof window.forceNavigationUpdate === "function") {
        window.forceNavigationUpdate();
      }
    }
  }

  // Logout: clear the cached signer alongside the server-side
  // session. navigation.js handles the POST to /api/v1/auth/logout
  // and the nav refresh; we just hook the same event to drop our
  // signer reference so the next login can't accidentally inherit
  // it.
  window.addEventListener("grain:logout", () => {
    try {
      window.grainSigner?.disconnect?.();
    } catch (_) {}
    // Wipe mill's persisted restore state (encrypted nsec, perms,
    // bunker connection) so the next page load can't silently
    // restore the signer for the account we just logged out of.
    try {
      window.MILL?.clearRestoreState?.();
    } catch (_) {}
    window.grainSigner = null;
    window.grainSignerMethod = null;
  });

  window.showAuthModal = showAuthModal;
  window.hideAuthModal = hideAuthModal;

  // ── Auto-reconnect ──────────────────────────────────────────
  //
  // window.grainSigner is a runtime JS object and doesn't survive a
  // page reload, but the server session cookie does and it knows the
  // signing method + pubkey. Mill persists each method's restore
  // state in sessionStorage, so MILL.restore({ method, pubkey })
  // rebuilds the signer for ANY method without re-opening the picker:
  //   - nip07:        rebuilt from window.nostr
  //   - bunker:       re-attaches the saved NIP-46 client
  //   - encrypted:    builds a lazy signer; prompts for the password
  //                   only when the first signEvent actually runs
  //   - amber:        reconstructed from pubkey + callback
  // If mill has no persisted state (e.g. the tab was closed and
  // sessionStorage cleared), restore returns null and the caller
  // falls back to the reconnect pill → MILL.open().
  //
  // restoreSigner is exposed on window so grain's ensureSigner can
  // call it on-demand before a save, not just on page load.

  let sessionCache = null;
  async function getCachedSession() {
    if (sessionCache !== null) return sessionCache;
    try {
      const r = await fetch("/api/v1/session", { cache: "no-store" });
      sessionCache = r.ok ? await r.json() : false;
    } catch (_) {
      sessionCache = false;
    }
    return sessionCache;
  }

  async function restoreSigner() {
    if (window.grainSigner && typeof window.grainSigner.signEvent === "function") return true;
    if (!window.MILL || typeof window.MILL.restore !== "function") return false;
    const sess = await getCachedSession();
    if (!sess || !sess.publicKey || !sess.signingMethod) return false;
    try {
      // MILL.restore accepts grain's SigningMethod enum directly.
      const signer = await window.MILL.restore({
        method: sess.signingMethod,
        pubkey: sess.publicKey,
      });
      if (!signer || typeof signer.signEvent !== "function") return false;
      window.grainSigner = signer;
      window.grainSignerMethod = signer.method || sess.signingMethod;
      // Notify listeners (admin dashboard's reconnect indicator)
      // that the signer is back without a mill picker round-trip.
      window.dispatchEvent(new CustomEvent("grain:signer-ready"));
      return true;
    } catch (_) {
      return false;
    }
  }
  window.restoreSigner = restoreSigner;

  // Initial reconnect on page load. NIP-07 extensions inject
  // window.nostr asynchronously, so for that method we poll a few
  // times (total budget ~3s). Other methods restore from mill's
  // persisted state, which is available immediately — a single
  // attempt is enough, and retrying could spin up duplicate
  // connections, so we don't.
  async function autoReconnectLoop() {
    if (await restoreSigner()) return;
    const sess = await getCachedSession();
    if (!sess || sess.signingMethod !== "browser_extension") return;
    const delays = [100, 200, 400, 800, 1500];
    for (const d of delays) {
      await new Promise((r) => setTimeout(r, d));
      if (await restoreSigner()) return;
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", autoReconnectLoop);
  } else {
    autoReconnectLoop();
  }
})();
