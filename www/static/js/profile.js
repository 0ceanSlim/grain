/**
 * Profile page functionality
 * Handles profile data loading, display, and management
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
      // Load cached profile data
      return fetch("/api/v1/cache");
    })
    .then((response) => {
      console.log("Cache response status:", response.status);
      if (!response.ok) {
        throw new Error("Failed to load profile cache");
      }
      return response.json();
    })
    .then((cacheData) => {
      console.log("Cache data received:", cacheData);
      displayProfile(cacheData);
    })
    .catch((error) => {
      console.error("Profile load error:", error);
      showError(error.message);
    });
}

function displayProfile(data) {
  console.log("Displaying profile with data:", data);

  // Hide loading and show content
  document.getElementById("loading").classList.add("hidden");
  document.getElementById("profile-content").classList.remove("hidden");

  // Update profile information
  if (data.metadata && data.metadata.content) {
    console.log("Processing metadata content:", data.metadata.content);

    let profileContent;
    try {
      // Parse the content string which contains the actual profile data
      profileContent = JSON.parse(data.metadata.content);
      console.log("Parsed profile content:", profileContent);
    } catch (e) {
      console.error("Failed to parse metadata content:", e);
      profileContent = {};
    }

    // Update profile fields with the parsed content
    document.getElementById("profile-name").textContent =
      profileContent.display_name ||
      profileContent.displayName ||
      profileContent.name ||
      "Anonymous";
    document.getElementById("profile-about").textContent =
      profileContent.about || "No bio available";
    document.getElementById("profile-pubkey").textContent =
      data.publicKey || data.metadata.pubkey || "Unknown";

    // Update avatar if available
    if (profileContent.picture) {
      document.getElementById(
        "profile-avatar"
      ).innerHTML = `<img src="${profileContent.picture}" alt="Profile" class="object-cover w-full h-full rounded-full">`;
    }
  } else {
    console.log("No metadata.content found in response");
    // Set default values
    document.getElementById("profile-name").textContent = "Anonymous";
    document.getElementById("profile-about").textContent = "No bio available";
    document.getElementById("profile-pubkey").textContent =
      data.publicKey || "Unknown";
  }

  // Update relay information
  if (data.mailboxes) {
    console.log("Processing mailboxes:", data.mailboxes);

    const mailboxes = data.mailboxes; // Already parsed as object
    console.log("Mailboxes object:", mailboxes);

    // Update counts
    const readCount =
      (mailboxes.read?.length || 0) + (mailboxes.both?.length || 0);
    const writeCount =
      (mailboxes.write?.length || 0) + (mailboxes.both?.length || 0);
    const totalCount =
      (mailboxes.read?.length || 0) +
      (mailboxes.write?.length || 0) +
      (mailboxes.both?.length || 0);

    document.getElementById("relay-count").textContent = totalCount;
    document.getElementById("read-relay-count").textContent = readCount;
    document.getElementById("write-relay-count").textContent = writeCount;

    // Display relay lists
    const readRelays = [...(mailboxes.read || []), ...(mailboxes.both || [])];
    const writeRelays = [...(mailboxes.write || []), ...(mailboxes.both || [])];

    displayRelayList("read-relays", readRelays);
    displayRelayList("write-relays", writeRelays);
  } else {
    console.log("No mailboxes found in response");
    // Set default values
    document.getElementById("relay-count").textContent = "0";
    document.getElementById("read-relay-count").textContent = "0";
    document.getElementById("write-relay-count").textContent = "0";

    displayRelayList("read-relays", []);
    displayRelayList("write-relays", []);
  }
}

function displayRelayList(elementId, relays) {
  const container = document.getElementById(elementId);
  if (!relays || relays.length === 0) {
    container.innerHTML = '<p class="text-gray-400">No relays configured</p>';
    return;
  }

  container.innerHTML = relays
    .map(
      (relay) =>
        `<div class="px-3 py-1 font-mono text-xs bg-gray-600 rounded">${relay}</div>`
    )
    .join("");
}

function refreshProfile() {
  console.log("Refreshing profile...");
  // Show loading state
  document.getElementById("loading").classList.remove("hidden");
  document.getElementById("profile-content").classList.add("hidden");
  document.getElementById("error-content").classList.add("hidden");

  // Reload data
  loadProfileData();
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

// Auto-load profile data when this script runs - only if we're on profile page or have profile elements
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
    fetch("/logout", { method: "POST" })
      .then((response) => {
        if (response.ok) {
          console.log("Logout successful");
          loadView("/views/home.html");
          updateNavigation();
        } else {
          console.error("Logout failed");
        }
      })
      .catch((error) => {
        console.error("Logout error:", error);
      });
  }
}
