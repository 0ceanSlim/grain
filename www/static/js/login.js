// Global state
let currentAuthMethod = null;

// Main modal functions
function showAuthModal() {
  document.getElementById("auth-modal").classList.remove("hidden");
  resetModal();
}

function hideAuthModal() {
  document.getElementById("auth-modal").classList.add("hidden");
  resetModal();
}

function resetModal() {
  // Hide all forms
  ["extension-form", "amber-form", "readonly-form", "privkey-form"].forEach(
    (id) => {
      document.getElementById(id).classList.add("hidden");
    }
  );

  // Show method selection
  document.getElementById("auth-method-selection").classList.remove("hidden");
  document.getElementById("close-button").classList.remove("hidden");

  // Reset advanced options
  document.getElementById("advanced-options").classList.add("hidden");
  document.getElementById("advanced-arrow").classList.remove("rotate-180");

  // Clear forms
  document.getElementById("bunker-url").value = "";
  document.getElementById("readonly-pubkey").value = "";
  document.getElementById("private-key").value = "";
  document.getElementById("session-password").value = "";

  // Clear results
  document.getElementById("auth-result").innerHTML = "";

  currentAuthMethod = null;
}

// Method selection
function selectAuthMethod(method) {
  currentAuthMethod = method;

  // Hide method selection
  document.getElementById("auth-method-selection").classList.add("hidden");
  document.getElementById("close-button").classList.add("hidden");

  // Show appropriate form
  const formId = method + "-form";
  document.getElementById(formId).classList.remove("hidden");

  // Special handling for extension
  if (method === "extension") {
    checkForExtension();
  }
}

function goBack() {
  resetModal();
}

// Advanced options toggle
function toggleAdvanced() {
  const advancedOptions = document.getElementById("advanced-options");
  const arrow = document.getElementById("advanced-arrow");

  if (advancedOptions.classList.contains("hidden")) {
    advancedOptions.classList.remove("hidden");
    arrow.classList.add("rotate-180");
  } else {
    advancedOptions.classList.add("hidden");
    arrow.classList.remove("rotate-180");
  }
}

// Extension handling
function checkForExtension() {
  const statusEl = document.getElementById("extension-status");
  const connectBtn = document.getElementById("connect-extension");

  if (window.nostr) {
    statusEl.innerHTML =
      '<div class="text-green-200">✅ Extension detected!</div>';
    statusEl.className =
      "p-3 mb-4 bg-green-800 border border-green-600 rounded-lg";
    connectBtn.disabled = false;
  } else {
    statusEl.innerHTML =
      '<div class="text-red-200">❌ No extension found. Please install Alby, nos2x, or another Nostr extension.</div>';
    statusEl.className = "p-3 mb-4 bg-red-800 border border-red-600 rounded-lg";
    connectBtn.disabled = true;
  }
}

async function connectExtension() {
  try {
    showAuthResult("loading", "Requesting access from extension...");

    const publicKey = await window.nostr.getPublicKey();

    // Create form data and submit
    const formData = new FormData();
    formData.append("publicKey", publicKey);
    formData.append("authMethod", "extension");

    const response = await fetch("/login", {
      method: "POST",
      body: formData,
    });

    if (response.ok) {
      showAuthResult("success", "Connected via extension!");
      setTimeout(() => {
        hideAuthModal();
        window.updateNavigation();
        // Navigate to profile
        setTimeout(() => {
          htmx.ajax("GET", "/views/profile.html", "#main-content");
          window.history.pushState({}, "", "/profile");
        }, 500);
      }, 1000);
    } else {
      const errorText = await response.text();
      showAuthResult("error", "Connection failed: " + errorText);
    }
  } catch (error) {
    showAuthResult("error", "Extension error: " + error.message);
  }
}

// Amber handling
function connectAmber() {
  const bunkerUrl = document.getElementById("bunker-url").value.trim();

  if (!bunkerUrl) {
    showAuthResult("error", "Please enter a bunker URL");
    return;
  }

  if (!bunkerUrl.startsWith("bunker://")) {
    showAuthResult("error", "Invalid bunker URL format");
    return;
  }

  showAuthResult("loading", "Connecting to Amber...");

  // TODO: Implement Amber connection logic
  // This would involve parsing the bunker URL and establishing NIP-46 connection
  setTimeout(() => {
    showAuthResult("error", "Amber integration coming soon!");
  }, 1000);
}

// Private key handling
function connectPrivateKey() {
  const privateKey = document.getElementById("private-key").value.trim();
  const sessionPassword = document.getElementById("session-password").value;

  if (!privateKey || !sessionPassword) {
    showAuthResult("error", "Please fill in all fields");
    return;
  }

  showAuthResult("loading", "Encrypting and storing key...");

  // TODO: Implement private key encryption and storage
  // This would involve:
  // 1. Validating the private key format
  // 2. Deriving the public key
  // 3. Encrypting the private key with the session password
  // 4. Storing encrypted key in session/memory
  // 5. Creating session with derived public key

  setTimeout(() => {
    showAuthResult("error", "Private key authentication coming soon!");
  }, 1000);
}

// Result display helper
function showAuthResult(type, message) {
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

  const resultDiv = document.getElementById("auth-result");
  if (resultDiv) {
    resultDiv.innerHTML = `<div class="${className} border px-4 py-3 rounded"><p>${icon} ${message}</p></div>`;
  }
}

// Expose functions globally for modal triggering
window.showAuthModal = showAuthModal;
window.hideAuthModal = hideAuthModal;
