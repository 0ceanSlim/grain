/**
 * Navigation management functions
 * Handles dynamic navigation updates based on user session
 */

// Global navigation update function
window.updateNavigation = function () {
  console.log("Updating navigation...");

  fetch("/api/v1/session")
    .then((response) => {
      console.log("Session check response status:", response.status);
      if (response.ok) {
        console.log("Session found, showing profile nav");
        updateNavToLoggedIn();
      } else {
        console.log("No session, showing login nav");
        updateNavToLoggedOut();
      }
    })
    .catch((error) => {
      console.error("Navigation update error:", error);
      updateNavToLoggedOut();
    });
};

// Force navigation update with cache busting
window.forceNavigationUpdate = function () {
  console.log("🔄 forceNavigationUpdate called");

  // Add cache busting parameter
  const cacheBuster = Date.now();
  fetch(`/api/v1/session?_=${cacheBuster}`)
    .then((response) => {
      console.log("🔄 Force session check response status:", response.status);
      if (response.ok) {
        console.log("🔄 Force session found, showing profile nav");
        updateNavToLoggedIn();
      } else {
        console.log("🔄 Force no session, showing login nav");
        updateNavToLoggedOut();
      }
    })
    .catch((error) => {
      console.error("🔄 Force navigation update error:", error);
      updateNavToLoggedOut();
    });
};

// Navigate to current user's profile using npub
window.navigateToUserProfile = async function () {
  console.log("🔄 navigateToUserProfile called");

  try {
    // Get current session to get pubkey
    const sessionResponse = await fetch("/api/v1/session");
    if (!sessionResponse.ok) {
      throw new Error("Not logged in");
    }

    const sessionData = await sessionResponse.json();
    const pubkey = sessionData.publicKey;

    if (!pubkey) {
      throw new Error("No public key in session");
    }

    console.log("🔄 Converting pubkey to npub", { pubkey });

    // Convert pubkey to npub using the correct endpoint
    const convertResponse = await fetch(
      `/api/v1/keys/convert/public/${pubkey}`
    );

    if (!convertResponse.ok) {
      throw new Error("Failed to convert pubkey to npub");
    }

    const convertData = await convertResponse.json();

    if (convertData.error) {
      throw new Error(convertData.error);
    }

    const npub = convertData.npub;
    console.log("🔄 Successfully converted to npub", { npub });

    // Navigate to profile page
    const profileUrl = `/p/${npub}`;
    htmx.ajax("GET", "/views/components/profile-page.html", "#main-content");
    window.history.pushState({}, "", profileUrl);

    console.log("🔄 Navigated to user profile", { profileUrl });
  } catch (error) {
    console.error("🔄 Failed to navigate to user profile:", error);
    // Fallback to old profile page if something goes wrong
    htmx.ajax("GET", "/views/profile.html", "#main-content");
    window.history.pushState({}, "", "/profile");
  }
};

// Update navigation to logged in state
function updateNavToLoggedIn() {
  console.log("🔄 updateNavToLoggedIn called");

  // Get user dropdown from template
  const dropdownTemplate = document.querySelector("#user-dropdown-template");
  if (!dropdownTemplate) {
    console.error("🔄 User dropdown template not found");
    console.log(
      "🔄 Available templates:",
      document.querySelectorAll('[id*="template"]')
    );
    return;
  }

  console.log("🔄 Found dropdown template, cloning");
  const userDropdown = dropdownTemplate.cloneNode(true);

  // Remove the template ID to avoid conflicts
  userDropdown.removeAttribute("id");

  // Clear and update profile nav
  const profileNav = document.getElementById("profile-nav");
  if (profileNav) {
    console.log("🔄 Updating profile-nav with dropdown");
    profileNav.innerHTML = "";
    profileNav.appendChild(userDropdown);

    // Process HTMX attributes on new dropdown
    if (typeof htmx !== "undefined") {
      htmx.process(profileNav);
    }

    console.log("🔄 Navigation updated to logged in state with dropdown");
  } else {
    console.error("🔄 profile-nav element not found");
  }
}

// Update navigation to logged out state
function updateNavToLoggedOut() {
  // Get login button from template
  const loginTemplate = document.querySelector("#login-template");
  if (!loginTemplate) {
    console.error("Login template not found");
    return;
  }
  const loginButton = loginTemplate.querySelector("button").cloneNode(true);

  // Update profile nav
  const profileNav = document.getElementById("profile-nav");
  if (profileNav) {
    profileNav.innerHTML = "";
    profileNav.appendChild(loginButton);

    // Process HTMX attributes on new button
    if (typeof htmx !== "undefined") {
      htmx.process(profileNav);
    }

    console.log("Navigation updated to logged out state");
  }
}

// Global logout function for buttons to use
window.logoutUser = function () {
  if (confirm("Are you sure you want to logout?")) {
    fetch("/api/v1/auth/logout", { method: "POST" }).then((response) => {
      if (response.ok) {
        console.log("Logout successful, updating navigation");
        // Force update navigation immediately
        window.forceNavigationUpdate();

        htmx.ajax("GET", "/views/home.html", "#main-content");
        window.history.pushState({}, "", "/");

        // Additional navigation update after redirect
        setTimeout(() => {
          window.forceNavigationUpdate();
        }, 100);
      }
    });
  }
};

// Safe event listener setup
function setupEventListeners() {
  // Initialize navigation on page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      console.log("DOM loaded, updating navigation");
      window.updateNavigation();
    });
  } else {
    console.log("DOM already loaded, updating navigation");
    window.updateNavigation();
  }

  // Listen for custom updateNav events
  if (document.body) {
    document.body.addEventListener("updateNav", function () {
      console.log("Received updateNav event, force updating");
      window.forceNavigationUpdate();
    });

    // Listen for HTMX events
    document.body.addEventListener("htmx:afterSettle", function () {
      console.log("HTMX after settle, updating navigation");
      setTimeout(window.updateNavigation, 100);
    });
  } else {
    console.error("document.body not available for event listeners");
  }
}

// Setup event listeners safely
setupEventListeners();
