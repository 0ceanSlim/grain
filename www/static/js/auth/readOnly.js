/**
 * Read-Only Authentication
 * Allows users to browse with just their public key (no signing capabilities)
 */

/**
 * Connect in read-only mode using public key
 */
async function connectReadOnly() {
  const pubkeyInput = document.getElementById("readonly-pubkey");

  if (!pubkeyInput) {
    AuthBase.showResult("error", "Public key input not found");
    return;
  }

  const pubkey = pubkeyInput.value.trim();

  // Validate input presence
  if (!pubkey) {
    AuthBase.showResult("error", "Enter a public key");
    return;
  }

  // Validate public key format
  if (!AuthBase.validatePubkey(pubkey)) {
    AuthBase.showResult("error", "Invalid public key format");
    return;
  }

  AuthBase.showResult("loading", "Creating session...");

  try {
    // Normalize public key (convert npub to hex if needed)
    const normalizedPubkey = AuthBase.normalizePublicKey(pubkey);

    if (!normalizedPubkey) {
      throw new Error("Could not process public key");
    }

    // Create login data for read-only mode
    const loginData = AuthBase.createLoginData(
      normalizedPubkey,
      "none", // No signing capability
      "read_only" // Read-only mode
    );

    // Send login request
    const result = await AuthBase.sendLoginRequest(loginData);

    // Handle successful login
    AuthBase.handleSuccessfulLogin(result);
  } catch (error) {
    AuthBase.showResult("error", `Failed: ${error.message}`);
  }
}

/**
 * Validate public key input as user types
 * Provides real-time feedback on key format
 */
function validatePublicKeyInput() {
  const pubkeyInput = document.getElementById("readonly-pubkey");
  const connectBtn = document.getElementById("connect-readonly");

  if (!pubkeyInput || !connectBtn) {
    return;
  }

  const pubkey = pubkeyInput.value.trim();

  // Enable/disable connect button based on validation
  if (pubkey && AuthBase.validatePubkey(pubkey)) {
    connectBtn.disabled = false;
    pubkeyInput.classList.remove("border-red-500");
    pubkeyInput.classList.add("border-green-500");
  } else {
    connectBtn.disabled = true;
    pubkeyInput.classList.remove("border-green-500");
    if (pubkey) {
      // Only show red if there's input
      pubkeyInput.classList.add("border-red-500");
    }
  }
}

/**
 * Setup read-only form event listeners
 */
function setupReadOnlyForm() {
  const pubkeyInput = document.getElementById("readonly-pubkey");

  if (pubkeyInput) {
    // Add input validation on keyup
    pubkeyInput.addEventListener("input", validatePublicKeyInput);

    // Add enter key support
    pubkeyInput.addEventListener("keypress", (e) => {
      if (e.key === "Enter") {
        connectReadOnly();
      }
    });
  }
}

// Initialize form when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", setupReadOnlyForm);
} else {
  setupReadOnlyForm();
}

// Export functions to global scope for HTML onclick handlers
window.connectReadOnly = connectReadOnly;

// Export AuthReadOnly object for programmatic access
window.AuthReadOnly = {
  connect: connectReadOnly,
  validateInput: validatePublicKeyInput,
  setupForm: setupReadOnlyForm,
};
