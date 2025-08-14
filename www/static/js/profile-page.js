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

    // Bio/about
    setElementText("profile-about", profileContent.about || "No bio available");

    // NIP-05 verification
    if (profileContent.nip05) {
      setElementText("profile-nip05", profileContent.nip05);
      showElement("profile-nip05-container");
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
