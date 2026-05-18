/**
 * Navigation: keep the header's login button in sync with /api/v1/session.
 *
 * The element #login-btn lives in the header at all times. This
 * script just updates its inner content + click handler depending
 * on whether there's an active session:
 *
 *   Logged out:  🗝️  + "Login"   →  click opens mill (window.showAuthModal)
 *   Logged in:   pfp + display    →  click toggles #user-dropdown-menu
 *
 * On mobile the text label is hidden by Tailwind (sm:inline), so the
 * button stays icon/pfp-only and never crowds the search toggle.
 */

(function () {
  "use strict";

  // ── Login button state ────────────────────────────────────────

  function getLoginBtn() {
    return document.getElementById("login-btn");
  }
  function getLoginBtnContent() {
    return document.getElementById("login-btn-content");
  }

  function renderLoggedOut() {
    const btn = getLoginBtn();
    const content = getLoginBtnContent();
    if (!btn || !content) return;
    btn.title = "Login";
    btn.disabled = false;
    btn.className =
      "flex items-center gap-2 px-3 py-2 text-sm font-medium border rounded bg-accent text-accent-fg border-accent hover:bg-accent-hover transition-colors";
    content.className = "inline-flex items-center gap-2";
    content.innerHTML =
      '<span>🗝️</span><span class="hidden sm:inline">Login</span>';
  }

  // Visible "logging you in" state used by the mill bridge while
  // the server roundtrips /api/v1/auth/login (which does a
  // synchronous mailbox + metadata fetch from outbox relays — see
  // grain issue tracker for the v0.8 outbox-model overhaul). The
  // existing nav refresh replaces this once /session + /cache come
  // back.
  function renderLoading() {
    const btn = getLoginBtn();
    const content = getLoginBtnContent();
    if (!btn || !content) return;
    btn.title = "Signing you in…";
    btn.disabled = true;
    btn.className =
      "flex items-center gap-2 px-3 py-2 text-sm font-medium border rounded bg-surface-elevated text-text-secondary border-border cursor-wait";
    content.className = "inline-flex items-center gap-2";
    content.innerHTML =
      '<span class="inline-block w-4 h-4 rounded-full animate-spin" style="border: 2px solid var(--color-accent-dim); border-top-color: var(--color-accent);"></span>' +
      '<span class="hidden sm:inline">Signing in…</span>';
  }
  window.renderLoginLoading = renderLoading;

  function renderLoggedIn(profileContent, npub) {
    const btn = getLoginBtn();
    const content = getLoginBtnContent();
    if (!btn || !content) return;
    const displayName =
      (profileContent &&
        (profileContent.display_name || profileContent.name)) ||
      (npub ? npub.slice(0, 12) + "…" : "User");

    // Switch to a quieter "logged in" look — the bright accent
    // button stops making sense once it's permanent chrome.
    btn.disabled = false;
    btn.className =
      "flex items-center gap-2 px-2 py-1 text-sm border rounded bg-surface-elevated text-text border-border hover:bg-surface-hover transition-colors";
    btn.title = displayName;
    // The img / avatar circle sits next to the display name as
    // sibling flex items so the pfp doesn't drop onto its own line.
    content.className = "inline-flex items-center gap-2";

    if (profileContent && profileContent.picture) {
      content.innerHTML =
        '<img src="' +
        escapeHtml(profileContent.picture) +
        '" alt="" class="w-6 h-6 rounded-full object-cover shrink-0" />' +
        '<span class="hidden sm:inline-block max-w-[12ch] truncate align-middle">' +
        escapeHtml(displayName) +
        "</span>";
    } else {
      content.innerHTML =
        '<span class="inline-flex items-center justify-center w-6 h-6 rounded-full bg-surface-overlay shrink-0">👤</span>' +
        '<span class="hidden sm:inline-block max-w-[12ch] truncate align-middle">' +
        escapeHtml(displayName) +
        "</span>";
    }
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  // ── Profile fetch ─────────────────────────────────────────────

  // /api/v1/cache returns the whole kind-0 EVENT under .metadata —
  // display_name / name / picture live inside its .content string as
  // a second JSON document (the same shape profile-page.js parses).
  // parseProfileContent surfaces the parsed inner object so callers
  // don't have to know about the nesting.
  function parseProfileContent(metadata) {
    if (!metadata) return null;
    if (typeof metadata.content === "string" && metadata.content) {
      try { return JSON.parse(metadata.content); } catch (_) { return null; }
    }
    // Tolerate already-flat shapes (server-side parse, or future API
    // change) so a reorder on the backend doesn't silently break us.
    if (metadata.display_name || metadata.name || metadata.picture) {
      return metadata;
    }
    return null;
  }

  async function fetchProfile() {
    try {
      const resp = await fetch("/api/v1/cache");
      if (!resp.ok) return null;
      const data = await resp.json();
      return {
        content: parseProfileContent(data.metadata),
        npub: data.npub || null,
        pubkey: data.publicKey || null,
      };
    } catch (e) {
      console.warn("[nav] failed to fetch profile", e);
      return null;
    }
  }

  // ── Dropdown ──────────────────────────────────────────────────

  function positionDropdown() {
    const menu = document.getElementById("user-dropdown-menu");
    const btn = getLoginBtn();
    if (!menu || !btn) return;
    const rect = btn.getBoundingClientRect();
    // Width matches Tailwind w-64 in the dropdown template.
    const menuWidth = 256;
    // Anchor the menu's right edge to the button's right edge so
    // the menu sits "below and aligned right" with the button.
    let left = rect.right - menuWidth;
    // Keep at least 8px from each viewport edge.
    left = Math.max(8, Math.min(left, window.innerWidth - menuWidth - 8));
    menu.style.left = left + "px";
    menu.style.top = rect.bottom + 8 + "px";
    // Clear any previous right/auto value left over from a different
    // viewport size; left wins from here.
    menu.style.right = "auto";
  }

  window.toggleUserDropdown = function () {
    const menu = document.getElementById("user-dropdown-menu");
    if (!menu) return;
    if (menu.classList.contains("hidden")) {
      positionDropdown();
      menu.classList.remove("hidden");
      // Refresh profile section every open so a new display
      // name / picture from /api/v1/cache shows up without a reload.
      fetchProfile().then(applyDropdownProfile);
      setTimeout(() => document.addEventListener("click", clickOutside), 0);
      window.addEventListener("resize", positionDropdown);
    } else {
      closeDropdown();
    }
  };
  window.closeUserDropdown = closeDropdown;

  function closeDropdown() {
    const menu = document.getElementById("user-dropdown-menu");
    if (menu) menu.classList.add("hidden");
    document.removeEventListener("click", clickOutside);
    window.removeEventListener("resize", positionDropdown);
  }

  function clickOutside(e) {
    const menu = document.getElementById("user-dropdown-menu");
    const btn = getLoginBtn();
    if (
      menu &&
      !menu.contains(e.target) &&
      btn &&
      !btn.contains(e.target)
    ) {
      closeDropdown();
    }
  }

  // Lazily-fetched + cached relay owner pubkey from NIP-11. We use
  // it for two things: the dropdown's admin-link reveal, and the
  // unclaimed-relay banner. Fetched once on load (kick-off below)
  // so the banner appears immediately rather than only after the
  // user opens the dropdown.
  let cachedRelayOwner = null;
  async function getRelayOwnerPubkey() {
    if (cachedRelayOwner !== null) return cachedRelayOwner;
    try {
      const resp = await fetch("/", {
        headers: { Accept: "application/nostr+json" },
      });
      if (!resp.ok) return (cachedRelayOwner = "");
      const info = await resp.json();
      cachedRelayOwner = (info && info.pubkey) || "";
    } catch (_) {
      cachedRelayOwner = "";
    }
    return cachedRelayOwner;
  }

  // Reveal the unowned-relay banner if NIP-11's pubkey field is
  // empty OR the all-zeros sentinel the example metadata ships with.
  // Runs once on page load so any visitor (logged in or not) sees
  // it. The banner element lives in templates/header.html, hidden
  // by default.
  const ALL_ZEROS_PUBKEY =
    "0000000000000000000000000000000000000000000000000000000000000000";
  async function maybeRevealUnownedBanner() {
    const banner = document.getElementById("unowned-banner");
    if (!banner) return;
    const owner = await getRelayOwnerPubkey();
    if (!owner || owner === ALL_ZEROS_PUBKEY) {
      banner.classList.remove("hidden");
    }
  }

  async function maybeRevealAdminLink(sessionPubkey) {
    const link = document.getElementById("user-dropdown-admin");
    if (!link || !sessionPubkey) return;
    const owner = await getRelayOwnerPubkey();
    if (
      owner &&
      owner !== ALL_ZEROS_PUBKEY &&
      owner.toLowerCase() === sessionPubkey.toLowerCase()
    ) {
      link.classList.remove("hidden");
      link.classList.add("flex");
    }
  }

  function applyDropdownProfile(info) {
    if (!info) return;
    const nameEl = document.getElementById("user-dropdown-name");
    const npubEl = document.getElementById("user-dropdown-npub");
    const pfpWrap = document.getElementById("user-dropdown-pfp-wrap");
    if (!nameEl || !npubEl || !pfpWrap) return;
    maybeRevealAdminLink(info.pubkey);
    const c = info.content || {};
    const display =
      c.display_name ||
      c.name ||
      (info.npub ? info.npub.slice(0, 12) + "…" : "User");
    nameEl.textContent = display;
    npubEl.textContent = info.npub
      ? info.npub.slice(0, 14) + "…" + info.npub.slice(-4)
      : "";
    if (c.picture) {
      pfpWrap.innerHTML =
        '<img src="' +
        escapeHtml(c.picture) +
        '" alt="" class="w-10 h-10 rounded-full object-cover" />';
    } else {
      pfpWrap.innerHTML = '<span class="text-lg">👤</span>';
    }
  }

  // ── Click router ──────────────────────────────────────────────

  // Single click handler the button always carries. Behaviour
  // depends on the current auth state cached on window.
  window.handleLoginClick = function () {
    if (window.__grainLoggedIn) {
      window.toggleUserDropdown();
    } else if (typeof window.showAuthModal === "function") {
      window.showAuthModal();
    } else {
      console.error("[nav] no login surface available");
    }
  };

  // ── Auth state sync ──────────────────────────────────────────

  window.updateNavigation = async function () {
    try {
      const resp = await fetch("/api/v1/session");
      if (!resp.ok) {
        window.__grainLoggedIn = false;
        renderLoggedOut();
        closeDropdown();
        return;
      }
      window.__grainLoggedIn = true;
      // Render with what we know from /session first (instant feedback)
      // then upgrade with metadata from /cache.
      renderLoggedIn(null, null);
      const info = await fetchProfile();
      if (info) renderLoggedIn(info.content, info.npub);
    } catch (e) {
      console.error("[nav] updateNavigation error", e);
      window.__grainLoggedIn = false;
      renderLoggedOut();
      closeDropdown();
    }
  };

  // Force update with cache-busting — used by logout flow + the
  // mill bridge right after a successful /api/v1/auth/login POST,
  // where the session cookie has just been set and we need to skip
  // any stale 401 response cached by the browser.
  window.forceNavigationUpdate = function () {
    const url = "/api/v1/session?_=" + Date.now();
    fetch(url)
      .then((r) => {
        if (r.ok) {
          window.__grainLoggedIn = true;
          renderLoggedIn(null, null);
          return fetchProfile().then((info) => {
            if (info) renderLoggedIn(info.content, info.npub);
          });
        } else {
          window.__grainLoggedIn = false;
          renderLoggedOut();
          closeDropdown();
        }
      })
      .catch(() => {
        window.__grainLoggedIn = false;
        renderLoggedOut();
        closeDropdown();
      });
  };

  // Navigate to the logged-in user's profile page. Existing
  // user-dropdown menu binds this.
  window.navigateToUserProfile = async function () {
    try {
      const sessionResp = await fetch("/api/v1/session");
      if (!sessionResp.ok) throw new Error("not logged in");
      const session = await sessionResp.json();
      const pubkey = session.publicKey;
      if (!pubkey) throw new Error("no public key");
      const convertResp = await fetch(
        "/api/v1/keys/convert/public/" + pubkey
      );
      if (!convertResp.ok) throw new Error("npub conversion failed");
      const conv = await convertResp.json();
      if (conv.error || !conv.npub) throw new Error(conv.error || "no npub");
      const npub = conv.npub;
      htmx.ajax("GET", "/views/components/profile-page.html", "#main-content");
      window.history.pushState({}, "", "/p/" + npub);
    } catch (e) {
      console.error("[nav] navigateToUserProfile failed", e);
      htmx.ajax("GET", "/views/home.html", "#main-content");
      window.history.pushState({}, "", "/");
    }
  };

  // ── Logout ───────────────────────────────────────────────────

  window.logoutUser = function () {
    if (!confirm("Are you sure you want to logout?")) return;
    fetch("/api/v1/auth/logout", { method: "POST" })
      .then((r) => {
        if (!r.ok) return;
        // Drop mill's signer reference alongside the server-side
        // session. The mill bridge listens for grain:logout and
        // handles its own cleanup.
        window.dispatchEvent(new CustomEvent("grain:logout"));
        window.forceNavigationUpdate();
        htmx.ajax("GET", "/views/home.html", "#main-content");
        window.history.pushState({}, "", "/");
        setTimeout(window.forceNavigationUpdate, 100);
      })
      .catch((e) => console.error("[nav] logout failed", e));
  };

  // ── Bootstrap ────────────────────────────────────────────────

  function bootstrap() {
    window.updateNavigation();
    maybeRevealUnownedBanner();
    document.body.addEventListener("updateNav", window.forceNavigationUpdate);
    document.body.addEventListener("htmx:afterSettle", function () {
      setTimeout(window.updateNavigation, 100);
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bootstrap);
  } else {
    bootstrap();
  }
})();
