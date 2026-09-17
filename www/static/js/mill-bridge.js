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
    // mill 1.7 Google logins. Stored under their own enum values so mill's
    // restore aliases (google→privatekey, pomegranate→nip46) rebuild the right
    // signer after a reload. See client/session/types.go:SigningMethod.
    google: "google",
    pomegranate: "pomegranate",
  };

  // The login-button template invokes `showAuthModal()` inline on
  // click. Keep that surface so we don't have to edit templates;
  // route it through mill.
  // The relay's NIP-11 identity (name + icon) brands the signer's custom header
  // and the appName shown to a remote signer / bunker when authorizing, so each
  // deployment identifies itself (e.g. "🌾 GRAIN Relay") rather than a generic
  // label. Cached after the first fetch; pre-warmed on load so it's ready by the
  // time the operator clicks login.
  let relayBrandCache = null;
  async function getRelayBrand() {
    if (relayBrandCache !== null) return relayBrandCache;
    try {
      const r = await fetch("/", { headers: { Accept: "application/nostr+json" } });
      const info = r.ok ? await r.json() : null;
      relayBrandCache = {
        name: (info && info.name) || "",
        icon: (info && info.icon) || "",
        terms: (info && info.terms_of_service) || "",
        privacy: (info && info.privacy_policy) || "",
      };
    } catch (_) {
      relayBrandCache = { name: "", icon: "", terms: "", privacy: "" };
    }
    return relayBrandCache;
  }
  getRelayBrand(); // pre-warm

  async function showAuthModal() {
    if (!window.MILL) {
      console.error(
        "[mill-bridge] MILL global not loaded — check /static/mill/mill.umd.min.js"
      );
      return;
    }
    const brand = await getRelayBrand();
    const appName = brand.name || document.title || "grain";

    // Terms / Privacy in the signer footer — only when THIS relay advertises
    // them in its NIP-11 (terms_of_service / privacy_policy). If neither is set,
    // we pass no footer and mill keeps its default "Signer by MILL" attribution.
    const footerLinks = [];
    if (brand.terms) footerLinks.push({ label: "Terms", href: brand.terms });
    if (brand.privacy) footerLinks.push({ label: "Privacy", href: brand.privacy });

    // mill 1.8 does its own platform detection (detectPlatform) and merges the
    // matching `platforms` block over the base opts, so grain no longer sniffs
    // the UA itself — it just declares desktop (base) + android/ios overrides.
    window.MILL.open({
      // Initial paint uses mill's grain theme; the CSS bridge takes over once
      // the element is in the DOM and renders.
      theme: "grain",
      // Grid picker (mill 1.8): the main methods render as tiles, not a list.
      layout: "grid",
      // Name the remote signer / bunker shows when authorizing (mill >= 1.2.0).
      appName,
      amberCallback: window.location.origin + "/api/v1/auth/amber-callback",
      onConnected: handleConnected,

      // Custom, relay-branded header (mill 1.8 per-field brand header). Title
      // comes from THIS relay's NIP-11 name, and the logo appears ONLY when the
      // relay advertises an image `icon` — so an operator brands the signer by
      // editing their relay's name/icon in admin, not by forking. No emoji logo
      // or eyebrow fallback (both `false` = hidden), so an unbranded relay shows
      // a clean name + message with no mill/grain defaults leaking in.
      header: {
        logo: brand.icon || false,
        logoHeight: brand.icon ? 40 : undefined,
        eyebrow: false,
        title: appName,
        message: "Choose how to connect your Nostr identity.",
        align: "center",
      },

      // "Google — Secure login" (pomegranate/FROST) via the default njump
      // ecosystem operators (auth.njump.me + four operators, 3-of-4). We never
      // set the <nostr-signer> `oauth-shim` attribute, so mill's other Google
      // path (Drive+PIN, method id `google`) stays hidden — that's the PIN
      // login we deliberately don't expose.
      pomegranate: true,

      // "New Identity" pinned to the top as a separated callout; with
      // pomegranate on it opens the "Continue with Google / generate keys"
      // chooser.
      callout: "newkey",

      // Drop mill's default footer tip ("NIP-07 browser extension is
      // recommended") — off-message when grain leads with Google. Operators can
      // set their own via config (see the signer-branding work).
      tip: false,

      // Relay's Terms / Privacy links, surfaced only when its NIP-11 advertises
      // them. Omitted entirely otherwise (keeps mill's default footer).
      footer: footerLinks.length ? { links: footerLinks } : undefined,

      // Desktop is the base layout: Google + browser extension as the two main
      // tiles, everything else tucked under a collapsed "More options" section
      // (mill 1.8). The per-platform blocks rearrange the mains; `google`
      // (Drive+PIN) is never listed, so the PIN login stays out of both.
      methods: ["pomegranate", "nip07"],
      moreMethods: ["nip46", "privatekey", "readonly"],
      platforms: {
        // Android: Google + Amber (NIP-55 intent — the right same-device path,
        // where NIP-46 over relays stalls on the backgrounded signer).
        android: {
          methods: ["pomegranate", "nip55"],
          moreMethods: ["nip46", "privatekey", "readonly"],
        },
        // iOS: Google + Private key (no NIP-07 extensions in iOS browsers).
        ios: {
          methods: ["pomegranate", "privatekey"],
          moreMethods: ["nip46", "readonly"],
        },
      },
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
