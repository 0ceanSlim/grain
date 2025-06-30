/**
 * Base authentication utilities
 * Common functions shared across all auth methods
 */

/**
 * Display result message to user with appropriate styling
 * @param {string} type - 'success', 'error', or 'loading'
 * @param {string} message - Message to display
 */
function showResult(type, message) {
  const colors = {
    success: "text-green-200 bg-green-800 border-green-600",
    error: "text-red-200 bg-red-800 border-red-600",
    loading: "text-blue-200 bg-blue-800 border-blue-600",
  };

  const icons = {
    success: "✅",
    error: "❌",
    loading: "⏳",
  };

  const html = `
      <div class="${colors[type] || colors.error} border px-4 py-3 rounded">
        <p>${icons[type] || icons.error} ${message}</p>
      </div>
    `;

  ui.setHtml("auth-result", html);
}

/**
 * Handle successful login response
 * Updates UI and redirects to profile
 * @param {Object} result - Login response from server
 */
function handleSuccessfulLogin(result) {
  showResult("success", "Login successful!");

  setTimeout(() => {
    hideAuthModal();
    updateNavigation();

    // Navigate to profile
    if (typeof htmx !== "undefined") {
      htmx.ajax("GET", "/views/profile.html", "#main-content");
      window.history.pushState({}, "", "/profile");
    }
  }, 1000);
}

/**
 * Send login request to server
 * @param {Object} loginData - Login data object
 * @returns {Promise<Object>} Server response
 */
async function sendLoginRequest(loginData) {
  try {
    const result = await api.post("/api/v1/auth/login", loginData);
    return result;
  } catch (error) {
    throw new Error(`Login failed: ${error.message}`);
  }
}

/**
 * Validate public key format (hex or npub)
 * @param {string} pubkey - Public key to validate
 * @returns {boolean} True if valid
 */
function validatePubkey(pubkey) {
  if (!pubkey || typeof pubkey !== "string") {
    return false;
  }

  const trimmed = pubkey.trim();

  // Check for npub format
  if (trimmed.startsWith("npub1")) {
    return trimmed.length === 63; // npub1 + 58 chars
  }

  // Check for hex format
  if (trimmed.length === 64) {
    return /^[0-9a-fA-F]{64}$/.test(trimmed);
  }

  return false;
}

/**
 * Convert npub to hex if needed
 * @param {string} pubkey - Public key in hex or npub format
 * @returns {string} Hex public key
 */
function normalizePublicKey(pubkey) {
  if (!pubkey) return "";

  const trimmed = pubkey.trim();

  // If already hex, return as-is
  if (trimmed.length === 64 && /^[0-9a-fA-F]{64}$/.test(trimmed)) {
    return trimmed;
  }

  // If npub, convert to hex (use existing validate utility)
  if (trimmed.startsWith("npub1")) {
    // This would use your existing conversion utility
    // For now, just validate it's the right format
    if (validate && validate.isValidNpub && validate.isValidNpub(trimmed)) {
      // Use existing conversion if available
      if (window.convertNpubToHex) {
        return window.convertNpubToHex(trimmed);
      }
    }
  }

  return trimmed;
}

/**
 * Clear authentication form inputs
 * @param {string[]} inputIds - Array of input element IDs to clear
 */
function clearAuthInputs(inputIds) {
  inputIds.forEach((id) => {
    const input = document.getElementById(id);
    if (input) {
      input.value = "";
    }
  });
}

/**
 * Create standardized login data object
 * @param {string} publicKey - User's public key (hex format)
 * @param {string} signingMethod - How events will be signed
 * @param {string} mode - 'read_write' or 'read_only'
 * @param {Object} extras - Additional fields specific to auth method
 * @returns {Object} Standardized login data
 */
function createLoginData(
  publicKey,
  signingMethod,
  mode = "read_write",
  extras = {}
) {
  return {
    public_key: publicKey,
    requested_mode: mode,
    signing_method: signingMethod,
    ...extras,
  };
}

// Export functions for use by other auth modules
window.AuthBase = {
  showResult,
  handleSuccessfulLogin,
  sendLoginRequest,
  validatePubkey,
  normalizePublicKey,
  clearAuthInputs,
  createLoginData,
};
