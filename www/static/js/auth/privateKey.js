/**
 * Private Key Authentication
 * Handles login with user's private key (encrypted for session storage)
 * TODO: Implement proper key encryption and session management
 */

/**
 * Connect using private key with session encryption
 */
async function connectPrivateKey() {
  const privateKeyInput = document.getElementById("private-key");
  const sessionPasswordInput = document.getElementById("session-password");

  if (!privateKeyInput || !sessionPasswordInput) {
    AuthBase.showResult("error", "Private key form not found");
    return;
  }

  const privateKey = privateKeyInput.value.trim();
  const sessionPassword = sessionPasswordInput.value.trim();

  // Validate inputs
  if (!privateKey) {
    AuthBase.showResult("error", "Enter your private key");
    return;
  }

  if (!sessionPassword) {
    AuthBase.showResult("error", "Enter a session password");
    return;
  }

  if (sessionPassword.length < 8) {
    AuthBase.showResult(
      "error",
      "Session password must be at least 8 characters"
    );
    return;
  }

  AuthBase.showResult("loading", "Processing private key...");

  try {
    // Validate and normalize private key
    const normalizedPrivateKey = normalizePrivateKey(privateKey);
    if (!normalizedPrivateKey) {
      throw new Error("Invalid private key format");
    }

    // Derive public key from private key
    const publicKey = await derivePublicKey(normalizedPrivateKey);
    if (!publicKey) {
      throw new Error("Could not derive public key");
    }

    // Encrypt private key for session storage
    const encryptedPrivateKey = await encryptPrivateKey(
      normalizedPrivateKey,
      sessionPassword
    );
    if (!encryptedPrivateKey) {
      throw new Error("Failed to encrypt private key");
    }

    // Create login data with encrypted private key
    const loginData = AuthBase.createLoginData(
      publicKey,
      "encrypted_key",
      "write",
      {
        encrypted_private_key: encryptedPrivateKey,
        // Don't send the session password to server
      }
    );

    // Send login request
    const result = await AuthBase.sendLoginRequest(loginData);

    // Clear sensitive inputs immediately
    privateKeyInput.value = "";
    sessionPasswordInput.value = "";

    // Handle successful login
    AuthBase.handleSuccessfulLogin(result);
  } catch (error) {
    AuthBase.showResult("error", `Failed: ${error.message}`);

    // Clear inputs on error for security
    privateKeyInput.value = "";
    sessionPasswordInput.value = "";
  }
}

/**
 * Normalize private key from various formats
 * @param {string} privateKey - Private key in hex or nsec format
 * @returns {string|null} Normalized hex private key or null if invalid
 */
function normalizePrivateKey(privateKey) {
  if (!privateKey || typeof privateKey !== "string") {
    return null;
  }

  const trimmed = privateKey.trim();

  // Check for nsec format
  if (trimmed.startsWith("nsec1")) {
    // TODO: Implement nsec to hex conversion
    // For now, just validate format
    if (trimmed.length === 63) {
      // Use existing conversion utility if available
      if (window.convertNsecToHex) {
        return window.convertNsecToHex(trimmed);
      }
    }
    return null; // Not implemented yet
  }

  // Check for hex format
  if (trimmed.length === 64 && /^[0-9a-fA-F]{64}$/.test(trimmed)) {
    return trimmed;
  }

  return null;
}

/**
 * Derive public key from private key
 * @param {string} privateKeyHex - Private key in hex format
 * @returns {Promise<string|null>} Public key in hex format
 */
async function derivePublicKey(privateKeyHex) {
  // TODO: Implement proper key derivation using secp256k1
  // This should:
  // 1. Parse private key hex to bytes
  // 2. Generate secp256k1 keypair
  // 3. Extract public key
  // 4. Return as hex string

  // For now, return null to indicate not implemented
  throw new Error("Public key derivation not yet implemented");
}

/**
 * Encrypt private key for session storage
 * @param {string} privateKeyHex - Private key in hex
 * @param {string} password - Session password
 * @returns {Promise<string|null>} Encrypted private key
 */
async function encryptPrivateKey(privateKeyHex, password) {
  // TODO: Implement client-side encryption
  // This should:
  // 1. Use WebCrypto API for encryption
  // 2. Derive key from password using PBKDF2
  // 3. Encrypt private key with AES-GCM
  // 4. Return base64 encrypted result

  throw new Error("Private key encryption not yet implemented");
}

/**
 * Sign event using encrypted private key
 * @param {Object} event - Event to sign
 * @param {string} sessionPassword - Password to decrypt private key
 * @returns {Promise<Object>} Signed event
 */
async function signEventWithPrivateKey(event, sessionPassword) {
  // TODO: Implement private key event signing
  // This should:
  // 1. Retrieve encrypted private key from session
  // 2. Decrypt using session password
  // 3. Sign event with decrypted key
  // 4. Clear decrypted key from memory
  // 5. Return signed event

  throw new Error("Private key event signing not yet implemented");
}

/**
 * Validate private key format as user types
 */
function validatePrivateKeyInput() {
  const privateKeyInput = document.getElementById("private-key");
  const sessionPasswordInput = document.getElementById("session-password");
  const connectBtn = document.getElementById("connect-private-key");

  if (!privateKeyInput || !sessionPasswordInput || !connectBtn) {
    return;
  }

  const privateKey = privateKeyInput.value.trim();
  const sessionPassword = sessionPasswordInput.value.trim();

  // Validate private key format
  const isValidKey = privateKey && normalizePrivateKey(privateKey) !== null;
  const isValidPassword = sessionPassword.length >= 8;

  // Update UI based on validation
  if (privateKey && !isValidKey) {
    privateKeyInput.classList.add("border-red-500");
  } else {
    privateKeyInput.classList.remove("border-red-500");
  }

  if (sessionPassword && !isValidPassword) {
    sessionPasswordInput.classList.add("border-red-500");
  } else {
    sessionPasswordInput.classList.remove("border-red-500");
  }

  // Enable connect button if both inputs are valid
  connectBtn.disabled = !(isValidKey && isValidPassword);
}

/**
 * Setup private key form event listeners
 */
function setupPrivateKeyForm() {
  const privateKeyInput = document.getElementById("private-key");
  const sessionPasswordInput = document.getElementById("session-password");

  if (privateKeyInput) {
    privateKeyInput.addEventListener("input", validatePrivateKeyInput);
  }

  if (sessionPasswordInput) {
    sessionPasswordInput.addEventListener("input", validatePrivateKeyInput);

    // Add enter key support
    sessionPasswordInput.addEventListener("keypress", (e) => {
      if (e.key === "Enter") {
        connectPrivateKey();
      }
    });
  }
}

/**
 * Clear all private key related data from memory
 */
function clearPrivateKeyData() {
  // Clear form inputs
  const privateKeyInput = document.getElementById("private-key");
  const sessionPasswordInput = document.getElementById("session-password");

  if (privateKeyInput) privateKeyInput.value = "";
  if (sessionPasswordInput) sessionPasswordInput.value = "";

  // TODO: Clear any cached decrypted keys from memory
}

// Initialize form when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", setupPrivateKeyForm);
} else {
  setupPrivateKeyForm();
}

// Export functions to global scope for HTML onclick handlers
window.connectPrivateKey = connectPrivateKey;

// Export AuthPrivateKey object for programmatic access
window.AuthPrivateKey = {
  connect: connectPrivateKey,
  signEvent: signEventWithPrivateKey,
  normalizePrivateKey,
  derivePublicKey,
  encryptPrivateKey,
  validateInput: validatePrivateKeyInput,
  setupForm: setupPrivateKeyForm,
  clearData: clearPrivateKeyData,
};
