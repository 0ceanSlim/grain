/**
 * Login page functionality
 * Handles login form validation and user feedback
 */

function showLoginResult(type, message) {
  let className, icon;

  if (type === "success") {
    className = "text-green-200 bg-green-800 border-green-600";
    icon = "✅";
  } else if (type === "error") {
    className = "text-red-200 bg-red-800 border-red-600";
    icon = "❌";
  } else if (type === "loading") {
    className = "text-blue-200 bg-blue-800 border-blue-600";
    icon = "⏳";
  } else {
    className = "text-gray-200 bg-gray-800 border-gray-600";
    icon = "ℹ️";
  }

  const resultDiv = document.getElementById("login-result");
  if (resultDiv) {
    resultDiv.innerHTML = `<div class="${className} border px-4 py-3 rounded"><p>${icon} ${message}</p></div>`;
  }
}

// Helper function for hex validation that Hyperscript can call
function isValidHex(value) {
  return /^[0-9a-fA-F]+$/.test(value);
}

// Handle login success and navigation
function handleLoginSuccess(responseText) {
  console.log("✅ Login successful, response:", responseText);

  showLoginResult("success", "Login successful! Redirecting...");

  // Simple redirect - let the server handle everything
  setTimeout(() => {
    console.log("🔄 Redirecting to /profile");
    window.location.assign("/profile");
  }, 1500);
}

// Direct DOM manipulation as fallback
function updateNavigationDirectly() {
  console.log("🔧 Attempting direct navigation update");

  const profileNav = document.getElementById("profile-nav");
  if (!profileNav) {
    console.error("🔧 profile-nav element not found");
    return;
  }

  console.log("🔧 Found profile-nav element");

  // Debug: List all available templates
  const allTemplates = document.querySelectorAll('[id*="template"]');
  console.log(
    "🔧 Available templates:",
    Array.from(allTemplates).map((t) => t.id)
  );

  // Try to get dropdown template
  const dropdownTemplate = document.querySelector("#user-dropdown-template");
  if (dropdownTemplate) {
    console.log("🔧 Found dropdown template, cloning it");
    const userDropdown = dropdownTemplate.cloneNode(true);
    userDropdown.removeAttribute("id");

    profileNav.innerHTML = "";
    profileNav.appendChild(userDropdown);

    // Process HTMX if available
    if (typeof htmx !== "undefined" && htmx.process) {
      htmx.process(profileNav);
      console.log("🔧 Processed HTMX attributes");
    }

    console.log(
      "🔧 Direct navigation update completed - dropdown should be visible"
    );
  } else {
    console.error(
      "🔧 Dropdown template not found, falling back to profile+logout buttons"
    );

    // Fallback to original profile + logout buttons
    const profileTemplate = document.querySelector("#profile-template");
    const logoutTemplate = document.querySelector("#logout-template");

    if (profileTemplate && logoutTemplate) {
      const profileButton = profileTemplate
        .querySelector("button")
        .cloneNode(true);
      const logoutButton = logoutTemplate
        .querySelector("button")
        .cloneNode(true);

      profileNav.innerHTML = "";
      profileNav.appendChild(profileButton);
      profileNav.appendChild(logoutButton);

      if (typeof htmx !== "undefined" && htmx.process) {
        htmx.process(profileNav);
      }

      console.log("🔧 Fallback to profile+logout buttons completed");
    } else {
      console.error("🔧 No suitable templates found for navigation update");
    }
  }
}

// Handle login error
function handleLoginError(xhr, message) {
  console.error("Login failed:", message, xhr);
  const errorMsg = message || "Login failed. Please check your public key.";
  showLoginResult("error", errorMsg);
}
