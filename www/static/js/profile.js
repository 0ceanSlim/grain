/**
 * Profile page functionality - Kind 1 profile data only
 * Handles display name, about, picture, banner, NIP-05, website, lightning
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

  // Store data globally for refresh functionality
  window.profileData = data;

  // Parse and display profile metadata (Kind 1 data only)
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

  // Update profile fields (Kind 1 data only)
  updateProfileFields(profileContent);

  // Update images (profile picture and banner)
  updateProfileImages(profileContent);
}

function updateProfileFields(profileContent) {
  // Basic profile info
  updateProfileField(
    "profile-name",
    profileContent.name || profileContent.display_name || "Unknown User"
  );
  updateProfileField(
    "profile-about",
    profileContent.about || "No bio available"
  );

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

// Utility functions
function updateProfileField(id, content) {
  const element = document.getElementById(id);
  if (element) {
    element.textContent = content;
  } else {
    console.warn(`Profile field element with id '${id}' not found`);
  }
}

function showElement(id) {
  const element = document.getElementById(id);
  if (element) {
    element.classList.remove("hidden");
  }
}

// Refresh profile function
function refreshProfile() {
  console.log("Refreshing profile...");

  // Show loading state
  document.getElementById("profile-content").classList.add("hidden");
  document.getElementById("loading").classList.remove("hidden");

  // Force cache refresh and reload data
  fetch("/api/v1/cache/refresh", { method: "POST" })
    .then((response) => {
      if (!response.ok) {
        throw new Error("Failed to refresh cache");
      }
      return response.json();
    })
    .then(() => {
      // Reload profile data after cache refresh
      loadProfileData();
      showNotification("Profile refreshed successfully!");
    })
    .catch((error) => {
      console.error("Profile refresh error:", error);
      showError("Failed to refresh profile: " + error.message);
    });
}

// Notification functions
function showNotification(message) {
  // Create a simple toast notification
  const notification = document.createElement("div");
  notification.className =
    "fixed z-50 px-4 py-2 text-white bg-green-600 rounded-lg shadow-lg top-4 right-4";
  notification.textContent = message;

  document.body.appendChild(notification);

  setTimeout(() => {
    notification.remove();
  }, 3000);
}

function showError(message) {
  console.error("Profile error:", message);

  // Hide loading and content, show error
  document.getElementById("loading").classList.add("hidden");
  document.getElementById("profile-content").classList.add("hidden");
  document.getElementById("error-content").classList.remove("hidden");

  // Update error message
  const errorElement = document.getElementById("error-message");
  if (errorElement) {
    errorElement.textContent = message;
  }
}

function showCacheError(message) {
  console.error("Cache error:", message);

  // Create a more informative error for cache issues
  const cacheMessage = `${message}. Click "Refresh Profile" to try updating the cache.`;
  showError(cacheMessage);
}

function showRefreshNotification() {
  showNotification("Profile data refreshed from relays");
}
