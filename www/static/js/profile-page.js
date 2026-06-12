(function () {
  // Encapsulate in IIFE to avoid conflicts with existing profile.js
  console.log("Profile component script starting");

  // Profile state
  let profileData = {
    identifier: "",
    pubkey: "",
    profile: null,
  };

  // Initialize profile page when component loads
  function initProfile() {
    console.log("Profile component loaded");

    // Extract identifier from URL
    const identifier = window.location.pathname.replace("/p/", "");
    if (!identifier) {
      showError("No profile identifier provided");
      return;
    }

    profileData.identifier = identifier;
    setElementText("profile-identifier", identifier);

    // Start the profile loading process
    loadProfile();
  }

  async function loadProfile() {
    try {
      console.log("Loading profile for:", profileData.identifier);

      // Step 1: Parse identifier to get pubkey
      const pubkey = await parseIdentifier(profileData.identifier);
      if (!pubkey) {
        throw new Error("Could not parse identifier");
      }

      profileData.pubkey = pubkey;
      setElementText("profile-pubkey", pubkey);

      // Step 2: Load profile data using existing API
      const profile = await fetchProfile(pubkey);
      if (profile) {
        profileData.profile = profile;
        displayProfile(profile);
      } else {
        throw new Error("Profile not found");
      }
    } catch (error) {
      console.error("Failed to load profile:", error);
      showError(error.message);
    } finally {
      hideElement("loading");
      showElement("profile-content");
    }
  }

  async function parseIdentifier(identifier) {
    // Handle different identifier formats
    if (identifier.startsWith("npub")) {
      // Convert npub to hex using existing API
      try {
        const response = await fetch(
          `/api/v1/keys/convert/public/${encodeURIComponent(identifier)}`
        );
        const data = await response.json();
        return data.public_key;
      } catch (error) {
        console.error("Failed to convert npub:", error);
        return null;
      }
    } else if (identifier.startsWith("nprofile")) {
      // TODO: nprofile parsing not implemented yet
      throw new Error("nprofile identifiers not yet supported");
    } else if (identifier.length === 64) {
      // Assume hex pubkey
      return identifier.toLowerCase();
    } else {
      throw new Error("Unrecognized identifier format");
    }
  }

  async function fetchProfile(pubkey) {
    try {
      // Use existing profile API
      const response = await fetch(
        `/api/v1/user/profile?pubkey=${encodeURIComponent(pubkey)}`
      );

      if (!response.ok) {
        throw new Error(`Profile API returned ${response.status}`);
      }

      const profile = await response.json();
      console.log("Profile data loaded:", profile);
      return profile;
    } catch (error) {
      console.error("Failed to fetch profile:", error);
      return null;
    }
  }

  function displayProfile(profile) {
    console.log("Profile component displaying profile:", profile);

    // Parse profile content (kind 0 metadata)
    let profileContent = {};

    // Handle different data structures
    let contentString = null;
    if (profile.content) {
      contentString = profile.content;
    } else if (profile.metadata && profile.metadata.content) {
      contentString = profile.metadata.content;
    }

    if (contentString) {
      try {
        profileContent = JSON.parse(contentString);
        console.log("Parsed profile content in component:", profileContent);
      } catch (e) {
        console.warn("Failed to parse profile content as JSON:", e);
        profileContent = { about: contentString };
      }
    } else {
      console.warn("No content found in profile data:", profile);
    }

    // Update profile fields
    updateProfileFields(profileContent);

    // Update images
    updateProfileImages(profileContent);

    // Update external identities from event tags (NIP-39)
    updateExternalIdentities(profile);

    // Keep the parsed content for the editor's diff, and reveal the Edit button
    // if this is the logged-in user's own profile.
    profileData.content = profileContent;
    checkOwnProfile();

    console.log("Profile component display complete");
  }

  function updateProfileFields(profileContent) {
    // Name and display name
    const name =
      profileContent.name || profileContent.display_name || "Unknown User";
    setElementText("profile-name", name);

    if (
      profileContent.display_name &&
      profileContent.display_name !== profileContent.name
    ) {
      setElementText(
        "profile-display-name",
        `"${profileContent.display_name}"`
      );
      showElement("profile-display-name");
    }

    // Bio/about - with clickable links
    setElementHTML(
      "profile-about",
      linkifyText(profileContent.about || "No bio available")
    );

    // NIP-05 verification with validation
    if (profileContent.nip05) {
      // Set initial loading state with spinner before the address
      setElementHTML(
        "profile-nip05",
        `<span class="inline-block w-3 h-3 border border-border-strong rounded-full animate-spin border-t-transparent mr-2"></span>${profileContent.nip05}`
      );
      showElement("profile-nip05-container");

      // Start verification
      verifyNip05(profileContent.nip05, profileData.pubkey);
    }

    // Website
    if (profileContent.website) {
      const websiteEl = document.getElementById("profile-website");
      websiteEl.href = profileContent.website;
      websiteEl.textContent = profileContent.website;
      showElement("profile-website-container");
    }

    // Lightning address (lud16 preferred; lud06 LNURL as fallback)
    const lightning = profileContent.lud16 || profileContent.lud06;
    if (lightning) {
      setElementText("profile-lightning", lightning);
      showElement("profile-lightning-container");
    }
  }

  // Verify NIP-05 identifier
  async function verifyNip05(nip05, expectedPubkey) {
    try {
      console.log("Verifying NIP-05:", nip05, "for pubkey:", expectedPubkey);

      // Parse the identifier
      const parts = nip05.split("@");
      if (parts.length !== 2) {
        throw new Error("Invalid NIP-05 format");
      }

      const [localPart, domain] = parts;

      // Make request to well-known endpoint
      const url = `https://${domain}/.well-known/nostr.json?name=${encodeURIComponent(
        localPart
      )}`;

      const response = await fetch(url);

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json();

      // Check if the names object exists and contains our local part
      if (!data.names || !data.names[localPart]) {
        throw new Error("Name not found in response");
      }

      const foundPubkey = data.names[localPart];

      // Compare pubkeys (normalize to lowercase)
      const isValid =
        foundPubkey.toLowerCase() === expectedPubkey.toLowerCase();

      updateNip05VerificationResult(
        isValid,
        isValid ? "Verified" : "Pubkey mismatch"
      );
    } catch (error) {
      console.warn("NIP-05 verification failed:", error);
      updateNip05VerificationResult(false, error.message);
    }
  }

  // Update the verification indicator with result
  function updateNip05VerificationResult(isValid, message) {
    const nip05Element = document.getElementById("profile-nip05");
    if (!nip05Element) return;

    // Get the original nip05 address (remove any existing verification indicator)
    const originalText = nip05Element.textContent
      .replace(/^[✅❌⏳]\s*/, "")
      .trim();

    if (isValid) {
      nip05Element.innerHTML = `<span class="text-success mr-2" title="NIP-05 verified">✅</span>${originalText}`;
    } else {
      nip05Element.innerHTML = `<span class="text-danger mr-2" title="NIP-05 verification failed: ${message}">❌</span>${originalText}`;
    }
  }

  // Update external identities from NIP-39 i tags
  function updateExternalIdentities(profile) {
    if (!profile.tags) return;

    // Find all 'i' tags
    const iTags = profile.tags.filter(
      (tag) => tag[0] === "i" && tag.length >= 3
    );

    if (iTags.length === 0) return;

    console.log("Found i tags:", iTags);

    // Process each supported platform
    const supportedPlatforms = [
      "github",
      "twitter",
      "x",
      "mastodon",
      "telegram",
    ];

    iTags.forEach((tag) => {
      const [, platformIdentity, proof] = tag;
      const [platform, identity] = platformIdentity.split(":");

      if (supportedPlatforms.includes(platform.toLowerCase())) {
        addExternalIdentityLink(platform.toLowerCase(), identity);
      }
    });
  }

  // Add external identity link to UI
  function addExternalIdentityLink(platform, identity) {
    const socialLinksContainer = document.querySelector(
      ".flex.justify-center.gap-6.mb-8"
    );
    if (!socialLinksContainer) return;

    // Platform configuration
    const platformConfig = {
      github: {
        name: "GitHub",
        icon: "https://github.githubassets.com/favicons/favicon-dark.png",
        getUrl: (identity) => `https://github.com/${identity}`,
      },
      mastodon: {
        name: "Mastodon",
        icon: "https://mastodon.social/packs/assets/favicon-16x16-74JBPGmr.png",
        getUrl: (identity) => `https://${identity}`,
      },
      x: {
        name: "X",
        icon: "https://abs.twimg.com/responsive-web/client-web/icon-svg.ea5ff4aa.svg",
        getUrl: (identity) => `https://x.com/${identity}`,
      },
      twitter: {
        name: "X",
        icon: "https://abs.twimg.com/responsive-web/client-web/icon-svg.ea5ff4aa.svg",
        getUrl: (identity) => `https://twitter.com/${identity}`,
      },
      telegram: {
        name: "Telegram",
        icon: "https://web.telegram.org/k/assets/img/favicon.ico",
        getUrl: (identity) => `https://t.me/${identity}`,
      },
    };

    const config = platformConfig[platform];
    if (!config) return;

    // Find or create platform element
    let platformElement = socialLinksContainer.querySelector(
      `[data-platform="${platform}"]`
    );

    if (!platformElement) {
      // Create new platform element
      platformElement = document.createElement("div");
      platformElement.setAttribute("data-platform", platform);

      // Replace the placeholder if it exists
      const placeholder = Array.from(socialLinksContainer.children).find(
        (child) => child.textContent.trim().toLowerCase() === platform
      );

      if (placeholder) {
        socialLinksContainer.replaceChild(platformElement, placeholder);
      } else {
        socialLinksContainer.appendChild(platformElement);
      }
    }

    // Create the link with icon and name
    const profileUrl = config.getUrl(identity);
    platformElement.innerHTML = `
        <a href="${profileUrl}" target="_blank" rel="noopener noreferrer" 
           class="inline-flex items-center gap-2 text-text-secondary hover:text-text transition-colors"
           title="${config.name} profile">
          <img src="${config.icon}" alt="${config.name}" class="w-4 h-4" />
          <span>${config.name}</span>
        </a>
      `;
  }

  function updateProfileImages(profileContent) {
    // Profile picture
    if (profileContent.picture) {
      const avatarImg = document.getElementById("profile-avatar-img");
      const avatarPlaceholder = document.getElementById(
        "profile-avatar-placeholder"
      );

      avatarImg.src = profileContent.picture;
      avatarImg.onload = function () {
        showElement("profile-avatar-img");
        hideElement("profile-avatar-placeholder");
      };
      avatarImg.onerror = function () {
        console.warn("Failed to load profile picture:", profileContent.picture);
      };
    }

    // Banner image
    if (profileContent.banner) {
      const bannerImg = document.getElementById("profile-banner-img");
      bannerImg.src = profileContent.banner;
      bannerImg.onload = function () {
        showElement("profile-banner");
      };
      bannerImg.onerror = function () {
        console.warn("Failed to load profile banner:", profileContent.banner);
      };
    }
  }

  // Function to convert URLs in text to clickable links and preserve line breaks
  function linkifyText(text) {
    // First, convert newlines to <br> tags
    let htmlText = text.replace(/\n/g, "<br>");

    // Then convert URLs to clickable links - improved regex to stop at whitespace or line breaks
    const urlRegex = /(https?:\/\/[^\s<]+)/g;

    return htmlText.replace(urlRegex, function (url) {
      // Remove trailing punctuation that might not be part of the URL
      const cleanUrl = url.replace(/[.,;:!?]+$/, "");
      const trailingPunc = url.substring(cleanUrl.length);

      return `<a href="${cleanUrl}" target="_blank" rel="noopener noreferrer" class="text-accent hover:text-accent underline">${cleanUrl}</a>${trailingPunc}`;
    });
  }

  // Action functions
  window.copyIdentifier = async function () {
    try {
      await navigator.clipboard.writeText(profileData.identifier);
      showToast("Profile identifier copied!");
    } catch (err) {
      console.error("Failed to copy identifier:", err);
      showToast("Failed to copy identifier", "error");
    }
  };

  window.copyPubkey = async function () {
    try {
      await navigator.clipboard.writeText(profileData.pubkey);
      showToast("Public key copied!");
    } catch (err) {
      console.error("Failed to copy pubkey:", err);
      showToast("Failed to copy public key", "error");
    }
  };

  window.refreshProfile = function () {
    // Reset state and reload
    hideElement("profile-content");
    hideElement("error");
    showElement("loading");
    loadProfile();
  };

  // Toggle the raw kind:0 event JSON in place (#75). Previously this button
  // called refreshProfile(), which re-fetched and re-rendered the whole
  // page instead of revealing the event — the opposite of "view event json".
  window.toggleEventJson = function () {
    const pre = document.getElementById("event-json");
    const btn = document.getElementById("event-json-btn");
    if (!pre) return;

    if (pre.classList.contains("hidden")) {
      const evt = profileData.profile;
      pre.textContent = evt
        ? JSON.stringify(evt, null, 2)
        : "No event loaded yet.";
      pre.classList.remove("hidden");
      if (btn) btn.textContent = "hide event json";
    } else {
      pre.classList.add("hidden");
      if (btn) btn.textContent = "view event json";
    }
  };

  // ── Profile editing (own profile only) ─────────────────────────
  // Editable kind-0 content fields. The input id is edit-<field> where <field>
  // is the content key, so we map both ways generically.
  const EDITABLE_FIELDS = [
    "name", "display_name", "about", "picture", "banner", "nip05", "lud16", "website",
  ];

  // Reveal the Edit button only when the logged-in user is viewing their own
  // profile (session pubkey === this profile's pubkey).
  async function checkOwnProfile() {
    try {
      const r = await fetch("/api/v1/session", { cache: "no-store" });
      if (!r.ok) return;
      const sess = await r.json();
      const mine =
        sess &&
        sess.publicKey &&
        sess.publicKey.toLowerCase() === (profileData.pubkey || "").toLowerCase();
      if (mine) {
        showElement("profile-edit-btn");
        showElement("profile-advanced-btn");
      }
    } catch (_) {
      /* not logged in — leave the Edit buttons hidden */
    }
  }

  window.toggleProfileEdit = function () {
    const panel = document.getElementById("profile-edit-panel");
    if (!panel) return;
    const opening = panel.classList.contains("hidden");
    if (opening) {
      const content = profileData.content || {};
      EDITABLE_FIELDS.forEach((f) => {
        const el = document.getElementById("edit-" + f);
        if (el) el.value = content[f] != null ? String(content[f]) : "";
      });
      setElementText("profile-save-status", "");
    }
    panel.classList.toggle("hidden");
  };

  window.saveProfile = async function () {
    const status = (m) => setElementText("profile-save-status", m);
    const content = profileData.content || {};

    // Collect ONLY the fields the user actually changed, so we add tags for
    // exactly those and leave everything else in the kind-0 untouched.
    const edits = {};
    EDITABLE_FIELDS.forEach((f) => {
      const el = document.getElementById("edit-" + f);
      if (!el) return;
      const newVal = el.value.trim();
      const oldVal = content[f] != null ? String(content[f]) : "";
      if (newVal !== oldVal) edits[f] = newVal;
    });
    if (Object.keys(edits).length === 0) {
      status("No changes to save.");
      return;
    }

    const saveBtn = document.getElementById("profile-save-btn");
    if (saveBtn) saveBtn.disabled = true;
    try {
      // Server merges the edits over the existing event → unsigned kind-0.
      status("Assembling event…");
      const buildResp = await fetch("/api/v1/user/profile/build", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ event: profileData.profile, edits }),
      });
      if (!buildResp.ok) throw new Error(await buildResp.text());
      const unsigned = await buildResp.json();
      await signAndPublish(unsigned, status, () => window.toggleProfileEdit());
    } catch (err) {
      console.error("Save profile failed:", err);
      status("Error: " + (err.message || err));
    } finally {
      if (saveBtn) saveBtn.disabled = false;
    }
  };

  // Shared: sign an unsigned event with the user's signer, publish via the
  // outbox, show the OK toast, and hydrate the page in place once a relay
  // accepts. Throws on failure (the caller surfaces the error).
  async function signAndPublish(unsigned, status, onAccepted) {
    status("Waiting for your signer…");
    if (typeof window.restoreSigner === "function") await window.restoreSigner();
    if (!window.grainSigner || typeof window.grainSigner.signEvent !== "function") {
      throw new Error("No signer available — log in with a signing method.");
    }
    const signed = await window.grainSigner.signEvent(unsigned);

    status(""); // the toast takes over from here
    const toast = makePublishToast(); // small bottom-right toast, spinner while sending

    let result;
    try {
      const pubResp = await fetch("/api/v1/events/publish", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ event: signed }),
      });
      result = await pubResp.json();
      if (!pubResp.ok || !result.success) {
        toast.fail(result.error || "publish failed");
        return;
      }
    } catch (err) {
      toast.fail(err.message || String(err));
      return;
    }

    toast.update(result);
    if ((result.results || []).some((r) => r.Accepted || r.accepted)) {
      // At least one relay stored it — hydrate the page in place from the event
      // we just signed; no reload.
      profileData.profile = signed;
      displayProfile(signed);
      if (onAccepted) onAccepted();
    }
  }

  // ── Advanced editor: full control over content fields and tags ──
  let advState = { content: [], tags: [] };
  let advDragFrom = null;

  function escAttr(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  window.toggleAdvancedEdit = function () {
    const panel = document.getElementById("profile-advanced-panel");
    if (!panel) return;
    const opening = panel.classList.contains("hidden");
    if (opening) {
      hideElement("profile-edit-panel"); // don't show both editors at once
      loadAdvState();
      setElementText("adv-status", "");
    }
    panel.classList.toggle("hidden");
  };

  function loadAdvState() {
    const evt = profileData.profile || {};
    const content = profileData.content || {};
    advState.content = Object.keys(content).map((k) => ({
      key: k,
      value: content[k] == null ? "" : String(content[k]),
    }));
    advState.tags = (evt.tags || []).map((t) => t.slice());
    const id = profileData.identifier || profileData.pubkey || "";
    setElementText("adv-meta", `kind 0 · ${id} · created_at: set on sign`);
    renderAdvContent();
    renderAdvTags();
  }

  function renderAdvContent() {
    const box = document.getElementById("adv-content");
    if (!box) return;
    box.innerHTML = "";
    advState.content.forEach((row, i) => {
      const div = document.createElement("div");
      div.className = "flex items-center gap-2";
      div.innerHTML =
        `<input value="${escAttr(row.key)}" placeholder="field" class="w-1/3 px-2 py-1 text-sm border rounded bg-surface-elevated text-text border-border" data-ck="${i}" />` +
        `<input value="${escAttr(row.value)}" placeholder="value" class="flex-1 px-2 py-1 text-sm border rounded bg-surface-elevated text-text border-border" data-cv="${i}" />` +
        `<button class="px-2 text-danger hover:opacity-80" data-cdel="${i}" title="Remove">×</button>`;
      box.appendChild(div);
    });
    box.querySelectorAll("[data-ck]").forEach((el) => {
      el.oninput = () => (advState.content[+el.dataset.ck].key = el.value);
    });
    box.querySelectorAll("[data-cv]").forEach((el) => {
      el.oninput = () => (advState.content[+el.dataset.cv].value = el.value);
    });
    box.querySelectorAll("[data-cdel]").forEach((el) => {
      el.onclick = () => {
        advState.content.splice(+el.dataset.cdel, 1);
        renderAdvContent();
      };
    });
  }

  function renderAdvTags() {
    const box = document.getElementById("adv-tags");
    if (!box) return;
    box.innerHTML = "";
    advState.tags.forEach((tag, i) => {
      const row = document.createElement("div");
      row.className = "flex items-center gap-2 p-1 rounded bg-surface-elevated";
      row.dataset.row = i;
      row.ondragover = (e) => e.preventDefault();
      row.ondrop = (e) => {
        e.preventDefault();
        advMoveTag(advDragFrom, i);
      };
      const els = tag
        .map(
          (el, j) =>
            `<input value="${escAttr(el)}" class="px-2 py-1 text-sm border rounded bg-surface text-text border-border" data-te="${i}_${j}" />`
        )
        .join("");
      row.innerHTML =
        `<span class="cursor-grab select-none text-text-secondary" draggable="true" title="Drag to reorder" data-grip="${i}">⠿</span>` +
        `<div class="flex flex-wrap items-center flex-1 gap-1">${els}` +
        `<button class="px-1 text-xs text-accent hover:opacity-80" data-teadd="${i}" title="Add element">+</button></div>` +
        `<button class="px-2 text-danger hover:opacity-80" data-tdel="${i}" title="Remove tag">×</button>`;
      box.appendChild(row);
    });
    box.querySelectorAll("[data-grip]").forEach((el) => {
      el.ondragstart = (e) => {
        advDragFrom = +el.dataset.grip;
        // Firefox won't start a drag unless some data is set.
        if (e.dataTransfer) e.dataTransfer.setData("text/plain", "");
      };
    });
    box.querySelectorAll("[data-te]").forEach((el) => {
      el.oninput = () => {
        const [i, j] = el.dataset.te.split("_").map(Number);
        advState.tags[i][j] = el.value;
      };
    });
    box.querySelectorAll("[data-teadd]").forEach((el) => {
      el.onclick = () => {
        advState.tags[+el.dataset.teadd].push("");
        renderAdvTags();
      };
    });
    box.querySelectorAll("[data-tdel]").forEach((el) => {
      el.onclick = () => {
        advState.tags.splice(+el.dataset.tdel, 1);
        renderAdvTags();
      };
    });
  }

  function advMoveTag(from, to) {
    if (from == null || from === to) return;
    const [item] = advState.tags.splice(from, 1);
    advState.tags.splice(to, 0, item);
    advDragFrom = null;
    renderAdvTags();
  }

  window.advAddContent = function () {
    advState.content.push({ key: "", value: "" });
    renderAdvContent();
  };
  window.advAddTag = function () {
    advState.tags.push(["", ""]);
    renderAdvTags();
  };

  window.advSave = async function () {
    const status = (m) => setElementText("adv-status", m);

    // Content: keep rows with a non-empty key, re-serialized to the content JSON.
    const content = {};
    advState.content.forEach((row) => {
      const k = (row.key || "").trim();
      if (k) content[k] = row.value;
    });

    // Tags: trim trailing empty elements, drop empty rows; order is preserved.
    const tags = advState.tags
      .map((t) => {
        const c = t.slice();
        while (c.length && c[c.length - 1] === "") c.pop();
        return c;
      })
      .filter((t) => t.length > 0);

    // Advanced = full control: assemble the event directly (no server merge).
    // created_at is stamped now; kind/pubkey are fixed.
    const unsigned = {
      kind: 0,
      pubkey: profileData.pubkey,
      created_at: Math.floor(Date.now() / 1000),
      content: JSON.stringify(content),
      tags: tags,
    };

    try {
      await signAndPublish(unsigned, status, () => window.toggleAdvancedEdit());
    } catch (err) {
      console.error("Advanced save failed:", err);
      status("Error: " + (err.message || err));
    }
  };

  // Publish toast: small, bottom-right. Spinner while sending; then the accepted
  // count. Click the header to expand per-relay details (accept / reject+reason
  // / send failure); dismiss anytime with the ×.
  function makePublishToast() {
    const t = document.createElement("div");
    t.className =
      "fixed z-50 overflow-hidden text-sm border rounded-lg shadow-lg bottom-4 right-4 w-72 bg-surface border-border";
    t.innerHTML =
      `<div class="flex items-center gap-2 px-3 py-2 cursor-pointer" data-head>` +
      `<span data-icon class="inline-block w-4 h-4 border-2 rounded-full border-text-secondary border-t-transparent animate-spin"></span>` +
      `<span data-title class="flex-1 font-medium text-text">Publishing…</span>` +
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
    };
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

  // Utility functions
  function showElement(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
      element.classList.remove("hidden");
    }
  }

  function hideElement(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
      element.classList.add("hidden");
    }
  }

  function setElementText(elementId, text) {
    const element = document.getElementById(elementId);
    if (element) {
      element.textContent = text;
    }
  }

  function setElementHTML(elementId, html) {
    const element = document.getElementById(elementId);
    if (element) {
      element.innerHTML = html;
    }
  }

  function showError(message) {
    setElementText("error-message", message);
    showElement("error");
    hideElement("loading");
  }

  function showToast(message, type = "success") {
    // Simple toast notification
    const toast = document.createElement("div");
    toast.className = `fixed top-4 right-4 px-4 py-2 rounded shadow-lg z-50 ${
      type === "error" ? "bg-danger text-text-on-accent" : "bg-success text-text-on-accent"
    }`;
    toast.textContent = message;

    document.body.appendChild(toast);

    setTimeout(() => {
      if (document.body.contains(toast)) {
        document.body.removeChild(toast);
      }
    }, 3000);
  }

  // Initialize when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initProfile);
  } else {
    initProfile();
  }
})();
