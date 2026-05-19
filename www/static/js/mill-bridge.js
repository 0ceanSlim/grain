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
  function showAuthModal() {
    if (!window.MILL) {
      console.error(
        "[mill-bridge] MILL global not loaded — check /static/mill/mill.umd.min.js"
      );
      return;
    }
    window.MILL.open({
      // Initial paint uses mill's grain theme; the CSS bridge takes
      // over once the element is in the DOM and renders.
      theme: "grain",
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
    window.grainSigner = null;
    window.grainSignerMethod = null;
  });

  window.showAuthModal = showAuthModal;
  window.hideAuthModal = hideAuthModal;

  // ── Auto-reconnect ──────────────────────────────────────────
  //
  // window.grainSigner is a runtime JS object and doesn't survive
  // a page reload, but the server session cookie does. For NIP-07
  // sign-ins we can transparently rebuild a signer from
  // window.nostr — extensions are always present once the page is
  // loaded.
  //
  // Two complications make this trickier than a single DOMContentLoaded
  // hook:
  //   1. Extensions inject window.nostr asynchronously. On a fast
  //      page load it may not be there yet. We poll for a few
  //      seconds after DOMContentLoaded.
  //   2. tryReconnectNIP07 also needs to be callable on-demand by
  //      grain's ensureSigner so first-save-after-reload doesn't
  //      need a retry from above. Exposed via window.tryReconnectNIP07.
  //
  // Other methods (bunker / encrypted / amber) keep their state
  // in mill — restoring them needs a mill-side rehydrate, tracked
  // in issue #81.

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

  async function tryReconnectNIP07() {
    if (window.grainSigner && typeof window.grainSigner.signEvent === "function") return true;
    if (!window.nostr || typeof window.nostr.signEvent !== "function") return false;
    const sess = await getCachedSession();
    if (!sess || !sess.publicKey) return false;
    if (sess.signingMethod !== "browser_extension") return false;
    try {
      const ext = await window.nostr.getPublicKey();
      if (!ext || ext.toLowerCase() !== sess.publicKey.toLowerCase()) return false;
      window.grainSigner = {
        getPublicKey: () => window.nostr.getPublicKey(),
        signEvent: (evt) => window.nostr.signEvent(evt),
        disconnect: () => {},
      };
      window.grainSignerMethod = "nip07";
      // Notify listeners (admin dashboard's reconnect indicator)
      // that the signer is back without a mill round-trip.
      window.dispatchEvent(new CustomEvent("grain:signer-ready"));
      return true;
    } catch (_) {
      return false;
    }
  }
  window.tryReconnectNIP07 = tryReconnectNIP07;

  // Initial reconnect loop: poll a few times so a slow-injecting
  // extension still gets caught. Total budget: ~3s with backoff.
  async function autoReconnectLoop() {
    if (await tryReconnectNIP07()) return;
    const delays = [100, 200, 400, 800, 1500];
    for (const d of delays) {
      await new Promise((r) => setTimeout(r, d));
      if (await tryReconnectNIP07()) return;
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", autoReconnectLoop);
  } else {
    autoReconnectLoop();
  }
})();
