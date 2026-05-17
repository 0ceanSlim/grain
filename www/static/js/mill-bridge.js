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

    const signingMethod = METHOD_MAP[result.method] ?? "none";
    const requestedMode = result.method === "readonly" ? "read_only" : "write";

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
      // "updateNav" CustomEvent — the listener (navigation.js:202)
      // binds to document.body and an event fired on window doesn't
      // bubble down to it. forceNavigationUpdate does a fresh,
      // cache-busted /api/v1/session check; same path the logout
      // flow uses.
      if (typeof window.forceNavigationUpdate === "function") {
        window.forceNavigationUpdate();
      }
      // Close the modal. Mill keeps the "connected" screen open by
      // default so the user can confirm; for grain we want to drop
      // straight into the dashboard. Short timeout gives mill's
      // animation room to finish.
      setTimeout(() => window.MILL?.close(), 300);
    } catch (err) {
      console.error("[mill-bridge] login request errored:", err);
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
})();
