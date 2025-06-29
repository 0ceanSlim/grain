/**
 * Complete Profile page functionality
 * Handles all profile fields, correct relay counting, and image loading
 */

function loadProfileData() {
  console.log("Loading profile data...");

  // First check if user has an active session
  fetch("/api/v1/session")
    .then((response) => {
      console.log("Session response status:", response.status);
      if (!response.ok) {
        throw new Error("No active session");
      }
      return response.json();
    })
    .then((sessionData) => {
      console.log("Session data:", sessionData);
      // Load cached profile data (will auto-refresh if expired)
      return fetch("/api/v1/cache");
    })
    .then((response) => {
      console.log("Cache response status:", response.status);

      if (response.status === 503) {
        // Service unavailable - cache refresh failed
        return response.json().then((errorData) => {
          throw new Error(errorData.message || "Cache refresh failed");
        });
      }

      if (!response.ok) {
        throw new Error("Failed to load profile cache");
      }

      return response.json();
    })
    .then((cacheData) => {
      console.log("Cache data received:", cacheData);

      // Show refresh indicator if data was just refreshed
      if (cacheData.refreshed) {
        showRefreshNotification();
      }

      displayProfile(cacheData);
    })
    .catch((error) => {
      console.error("Profile load error:", error);

      // Check if it's a cache-related error and offer refresh
      if (
        error.message.includes("Cache") ||
        error.message.includes("refresh")
      ) {
        showCacheError(error.message);
      } else {
        showError(error.message);
      }
    });
}

function displayProfile(data) {
  console.log("Displaying profile with data:", data);

  // Hide loading and show content
  document.getElementById("loading").classList.add("hidden");
  document.getElementById("profile-content").classList.remove("hidden");

  // Store data globally for copy functions
  window.profileData = data;

  // Update debug info
  updateDebugInfo(data);

  // Parse and display profile metadata
  let profileContent = {};
  if (data.metadata && data.metadata.content) {
    try {
      profileContent = JSON.parse(data.metadata.content);
      console.log("Parsed profile content:", profileContent);
    } catch (e) {
      console.log("Failed to parse content as JSON:", e);
      profileContent = { about: data.metadata.content };
    }
  }

  // Update all profile fields
  updateProfileFields(profileContent, data);

  // Update session information
  updateSessionInfo(data);

  // Update relay information and counts
  updateRelayInformation(data);

  // Update images (profile picture and banner)
  updateProfileImages(profileContent);
}

function updateProfileFields(profileContent, data) {
  // Basic profile info
  updateProfileField(
    "profile-name",
    profileContent.name || profileContent.display_name || "Unknown User"
  );
  updateProfileField(
    "profile-about",
    profileContent.about || "No bio available"
  );
  updateProfileField(
    "profile-pubkey",
    data.publicKey || data.metadata?.pubkey || "Not available"
  );
  updateProfileField("profile-npub", data.npub || "Not available");

  // Display name (if different from name)
  if (
    profileContent.display_name &&
    profileContent.display_name !== profileContent.name
  ) {
    updateProfileField(
      "profile-display-name",
      `"${profileContent.display_name}"`
    );
    showElement("profile-display-name");
  }

  // NIP-05 verification
  if (profileContent.nip05) {
    updateProfileField("profile-nip05", profileContent.nip05);
    showElement("profile-nip05-container");
  }

  // Website
  if (profileContent.website) {
    const websiteEl = document.getElementById("profile-website");
    if (websiteEl) {
      websiteEl.href = profileContent.website;
      websiteEl.textContent = profileContent.website;
    }
    showElement("profile-website-container");
  }

  // Lightning address
  if (profileContent.lud16) {
    updateProfileField("profile-lightning", profileContent.lud16);
    showElement("profile-lightning-container");
  }
}

function updateSessionInfo(data) {
  updateProfileField("session-mode", data.sessionMode || "unknown");
  updateProfileField("signing-method", data.signingMethod || "unknown");

  // Derive canCreateEvents from sessionMode instead of using separate field
  const canCreateEvents = data.sessionMode === "write";
  updateProfileField("can-create-events", canCreateEvents ? "Yes" : "No");

  updateProfileField("cache-age", data.cacheAge || "unknown");
}

function updateProfileImages(profileContent) {
  // Profile picture
  if (profileContent.picture) {
    const profilePic = document.getElementById("profile-picture");
    const fallback = document.getElementById("profile-picture-fallback");

    if (profilePic && fallback) {
      profilePic.src = profileContent.picture;
      profilePic.onload = function () {
        fallback.style.display = "none";
        profilePic.style.display = "block";
      };
      profilePic.onerror = function () {
        fallback.style.display = "flex";
        profilePic.style.display = "none";
      };
    }
  }

  // Banner image
  if (profileContent.banner) {
    const bannerContainer = document.getElementById("profile-banner-container");
    const banner = document.getElementById("profile-banner");

    if (banner && bannerContainer) {
      banner.src = profileContent.banner;
      banner.onload = function () {
        bannerContainer.classList.remove("hidden");
      };
      banner.onerror = function () {
        bannerContainer.classList.add("hidden");
      };
    }
  }
}

function updateRelayInformation(data) {
  let readOnlyRelays = [];
  let writeOnlyRelays = [];
  let bothRelays = [];

  // Extract relay information from the response
  if (data.mailboxes || data.relayInfo) {
    const relayData = data.relayInfo || data.mailboxes;

    readOnlyRelays = relayData.read || [];
    // Note: writeOnlyRelays removed since we eliminated the redundant write field
    // writeOnlyRelays = relayData.write || []; // This was always null/empty
    bothRelays = relayData.both || [];

    console.log("Relay data extracted:", {
      readOnly: readOnlyRelays,
      both: bothRelays,
    });
  }

  // Update relay lists
  updateRelayList("read-relays", readOnlyRelays);
  updateRelayList("write-relays", []); // Clear write-only relays since we removed that field
  updateRelayList("both-relays", bothRelays);

  // Calculate and update relay counts
  const readRelayCount = readOnlyRelays.length + bothRelays.length;
  const writeRelayCount = bothRelays.length; // Only count 'both' relays for write
  const totalRelayCount = readOnlyRelays.length + bothRelays.length;

  updateProfileField("read-relay-count", readRelayCount.toString());
  updateProfileField("write-relay-count", writeRelayCount.toString());
  updateProfileField("relay-count", totalRelayCount.toString());

  console.log("Relay counts updated:", {
    read: readRelayCount,
    write: writeRelayCount,
    total: totalRelayCount,
  });
}

function updateProfileField(elementId, value) {
  const element = document.getElementById(elementId);
  if (element) {
    element.textContent = value;
  } else {
    console.log(`Element ${elementId} not found`);
  }
}

function showElement(elementId) {
  const element = document.getElementById(elementId);
  if (element) {
    element.classList.remove("hidden");
  }
}

function updateRelayList(elementId, relays) {
  const element = document.getElementById(elementId);
  if (!element) {
    console.log(`Relay list element ${elementId} not found`);
    return;
  }

  if (!relays || relays.length === 0) {
    element.innerHTML = '<p class="text-gray-400">No relays configured</p>';
    return;
  }

  element.innerHTML = relays
    .map((relay) => {
      // Clean up the relay URL for display
      const displayRelay = relay.replace(/^wss?:\/\//, "").replace(/\/$/, "");
      return `<div class="px-3 py-1 font-mono text-xs bg-gray-600 rounded hover:bg-gray-500 transition-colors" title="${relay}">${displayRelay}</div>`;
    })
    .join("");

  console.log(`Updated ${elementId} with ${relays.length} relays`);
}

function updateDebugInfo(data) {
  const debugEl = document.getElementById("debug-data");
  if (debugEl) {
    debugEl.textContent = JSON.stringify(data, null, 2);
  }
}

function refreshProfile() {
  console.log("Manual profile refresh...");

  // Show loading state
  document.getElementById("loading").classList.remove("hidden");
  document.getElementById("profile-content").classList.add("hidden");
  document.getElementById("error-content").classList.add("hidden");

  // Call manual refresh endpoint first
  fetch("/api/v1/cache/refresh", {
    method: "POST",
  })
    .then((response) => response.json())
    .then((result) => {
      console.log("Manual refresh result:", result);
      if (result.success) {
        // Now reload the profile data
        loadProfileData();
      } else {
        throw new Error(result.message || "Manual refresh failed");
      }
    })
    .catch((error) => {
      console.error("Manual refresh failed:", error);
      // Fall back to normal load which will try auto-refresh
      loadProfileData();
    });
}

// Copy functions
function copyPublicKey() {
  if (window.profileData?.publicKey) {
    navigator.clipboard
      .writeText(window.profileData.publicKey)
      .then(() => {
        showNotification("Public key copied to clipboard!", "success");
      })
      .catch((err) => {
        console.error("Failed to copy public key:", err);
        showNotification("Failed to copy public key", "error");
      });
  }
}

function copyNpub() {
  if (window.profileData?.npub) {
    navigator.clipboard
      .writeText(window.profileData.npub)
      .then(() => {
        showNotification("npub copied to clipboard!", "success");
      })
      .catch((err) => {
        console.error("Failed to copy npub:", err);
        showNotification("Failed to copy npub", "error");
      });
  }
}

function showNotification(message, type = "info") {
  const notification = document.createElement("div");

  let bgClass, borderClass, textClass;
  switch (type) {
    case "success":
      bgClass = "bg-green-800";
      borderClass = "border-green-600";
      textClass = "text-green-200";
      break;
    case "error":
      bgClass = "bg-red-800";
      borderClass = "border-red-600";
      textClass = "text-red-200";
      break;
    default:
      bgClass = "bg-blue-800";
      borderClass = "border-blue-600";
      textClass = "text-blue-200";
  }

  notification.className = `fixed top-4 right-4 ${bgClass} border ${borderClass} ${textClass} px-4 py-2 rounded-lg z-50`;
  notification.textContent = message;

  document.body.appendChild(notification);

  // Remove after 3 seconds
  setTimeout(() => {
    if (notification.parentNode) {
      notification.parentNode.removeChild(notification);
    }
  }, 3000);
}

function showRefreshNotification() {
  showNotification("✅ Profile data refreshed", "success");
}

function showCacheError(message) {
  console.log("Showing cache error:", message);
  const loadingEl = document.getElementById("loading");
  const profileContentEl = document.getElementById("profile-content");
  const errorContentEl = document.getElementById("error-content");
  const errorMessageEl = document.getElementById("error-message");

  if (loadingEl) loadingEl.classList.add("hidden");
  if (profileContentEl) profileContentEl.classList.add("hidden");
  if (errorContentEl) errorContentEl.classList.remove("hidden");

  if (errorMessageEl) {
    errorMessageEl.innerHTML = `
      <div class="space-y-3">
        <p>${message}</p>
        <div class="text-sm text-gray-300">
          Your session is still active, but profile data needs to be refreshed.
        </div>
        <button 
          onclick="refreshProfile()" 
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded"
        >
          🔄 Refresh Now
        </button>
      </div>
    `;
  }
}

function showError(message) {
  console.log("Showing error:", message);
  const loadingEl = document.getElementById("loading");
  const profileContentEl = document.getElementById("profile-content");
  const errorContentEl = document.getElementById("error-content");
  const errorMessageEl = document.getElementById("error-message");

  if (loadingEl) loadingEl.classList.add("hidden");
  if (profileContentEl) profileContentEl.classList.add("hidden");
  if (errorContentEl) errorContentEl.classList.remove("hidden");
  if (errorMessageEl) errorMessageEl.textContent = message;
}

// Auto-load profile data when this script runs
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", function () {
    if (document.getElementById("profile-content")) {
      loadProfileData();
    }
  });
} else {
  if (document.getElementById("profile-content")) {
    loadProfileData();
  }
}

// Logout function
function logout() {
  if (confirm("Are you sure you want to logout?")) {
    fetch("/api/v1/auth/logout", { method: "POST" })
      .then((response) => response.json())
      .then((result) => {
        if (result.success) {
          console.log("Logout successful");
          // Navigate home
          if (typeof htmx !== "undefined") {
            htmx.ajax("GET", "/views/home.html", "#main-content");
          } else {
            window.location.href = "/";
          }
          // Update navigation
          if (window.updateNavigation) {
            window.updateNavigation();
          }
        } else {
          console.error("Logout failed:", result.message);
        }
      })
      .catch((error) => {
        console.error("Logout error:", error);
      });
  }
}

// Expose functions globally
window.loadProfileData = loadProfileData;
window.refreshProfile = refreshProfile;
window.copyPublicKey = copyPublicKey;
window.copyNpub = copyNpub;
window.logout = logout;
