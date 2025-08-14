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
        `<span class="inline-block w-3 h-3 border border-gray-400 rounded-full animate-spin border-t-transparent mr-2"></span>${profileContent.nip05}`
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

    // Lightning address
    if (profileContent.lud16) {
      setElementText("profile-lightning", profileContent.lud16);
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
      nip05Element.innerHTML = `<span class="text-green-400 mr-2" title="NIP-05 verified">✅</span>${originalText}`;
    } else {
      nip05Element.innerHTML = `<span class="text-red-400 mr-2" title="NIP-05 verification failed: ${message}">❌</span>${originalText}`;
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
           class="inline-flex items-center gap-2 text-gray-300 hover:text-white transition-colors"
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

      return `<a href="${cleanUrl}" target="_blank" rel="noopener noreferrer" class="text-blue-400 hover:text-blue-300 underline">${cleanUrl}</a>${trailingPunc}`;
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
      type === "error" ? "bg-red-600 text-white" : "bg-green-600 text-white"
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
