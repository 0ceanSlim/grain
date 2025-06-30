// Global state
let currentAuthMethod = null;
let amberCallbackReceived = false;

// Expose functions globally
window.showAuthModal = showAuthModal;
window.hideAuthModal = hideAuthModal;
window.signEventWithExtension = signEventWithExtension;
window.isExtensionAvailable = isExtensionAvailable;
window.isAmberAvailable = isAmberAvailable;
window.getExtensionPublicKey = getExtensionPublicKey;

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
  [
    "extension-form",
    "amber-form",
    "bunker-form",
    "readonly-form",
    "privkey-form",
  ].forEach((id) => {
    document.getElementById(id).classList.add("hidden");
  });

  // Show method selection
  document.getElementById("auth-method-selection").classList.remove("hidden");
  document.getElementById("close-button").classList.remove("hidden");

  // Reset advanced options
  document.getElementById("advanced-options").classList.add("hidden");
  document.getElementById("advanced-arrow").classList.remove("rotate-180");

  // Clear forms
  document.getElementById("bunker-url").value = "";
  if (document.getElementById("amber-bunker-url")) {
    document.getElementById("amber-bunker-url").value = "";
  }
  document.getElementById("readonly-pubkey").value = "";
  document.getElementById("private-key").value = "";
  document.getElementById("session-password").value = "";

  // Clear results
  document.getElementById("auth-result").innerHTML = "";

  currentAuthMethod = null;
  amberCallbackReceived = false;
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

// Advanced options toggle - FIXED
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

/**
 * Utility functions
 */
function isValidPublicKey(pubkey) {
  if (pubkey.startsWith("npub")) {
    return pubkey.length === 63; // npub1 + 58 chars
  }
  return /^[0-9a-fA-F]{64}$/.test(pubkey);
}

function npubToHex(npub) {
  // Let backend handle npub conversion
  return npub;
}

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
