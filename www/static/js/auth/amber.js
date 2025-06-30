/**
 * Amber Authentication (Android App)
 * Handles login via Amber signer app using intent-based communication
 * TODO: Implement proper Amber protocol integration
 */

/**
 * Connect via Amber app
 * Supports both direct connection and bunker URL
 */
async function connectAmber() {
  const bunkerUrlInput = document.getElementById("amber-bunker-url");

  if (!bunkerUrlInput) {
    AuthBase.showResult("error", "Amber form not found");
    return;
  }

  const bunkerUrl = bunkerUrlInput.value.trim();

  AuthBase.showResult("loading", "Connecting to Amber...");

  try {
    if (bunkerUrl) {
      // Use bunker URL if provided
      await connectAmberViaBunker(bunkerUrl);
    } else {
      // Direct Amber connection
      await connectAmberDirect();
    }
  } catch (error) {
    AuthBase.showResult("error", `Amber connection failed: ${error.message}`);
  }
}

/**
 * Connect to Amber using bunker URL
 * @param {string} bunkerUrl - Bunker URL from user input
 */
async function connectAmberViaBunker(bunkerUrl) {
  // Validate bunker URL format
  if (!bunkerUrl.startsWith("bunker://")) {
    throw new Error("Invalid bunker URL format");
  }

  // TODO: Implement NIP-46 bunker connection
  // This should:
  // 1. Parse bunker URL to extract pubkey and relay
  // 2. Connect to specified relay
  // 3. Send connection request to bunker
  // 4. Handle authentication challenge
  // 5. Create session with "amber" signing method

  throw new Error("Amber bunker connection not yet implemented");
}

/**
 * Connect to Amber directly (Android intent)
 */
async function connectAmberDirect() {
  // Check if we're on Android
  const isAndroid = /Android/i.test(navigator.userAgent);

  if (!isAndroid) {
    throw new Error("Direct Amber connection only works on Android devices");
  }

  // TODO: Implement direct Amber connection
  // This should:
  // 1. Create an Android intent to launch Amber
  // 2. Request public key from Amber
  // 3. Handle Amber response via intent callback
  // 4. Create session with Amber signing method

  throw new Error("Direct Amber connection not yet implemented");
}

/**
 * Sign event using Amber
 * @param {Object} event - Event to sign
 * @returns {Promise<Object>} Signed event
 */
async function signEventWithAmber(event) {
  // TODO: Implement Amber event signing
  // This should:
  // 1. Create signing request
  // 2. Send to Amber via intent or bunker
  // 3. Wait for signed response
  // 4. Return signed event

  throw new Error("Amber event signing not yet implemented");
}

/**
 * Check if Amber app is available on device
 * @returns {boolean} True if Amber is available
 */
function isAmberAvailable() {
  // TODO: Implement Amber availability check
  // This should check if Amber app is installed

  const isAndroid = /Android/i.test(navigator.userAgent);
  return isAndroid; // Placeholder
}

/**
 * Validate bunker URL format
 * @param {string} url - Bunker URL to validate
 * @returns {boolean} True if valid
 */
function validateBunkerUrl(url) {
  if (!url || typeof url !== "string") {
    return false;
  }

  // Basic bunker URL validation
  // Format: bunker://pubkey?relay=wss://relay.url
  const bunkerRegex = /^bunker:\/\/[0-9a-fA-F]{64}\?relay=wss?:\/\/.+$/;
  return bunkerRegex.test(url.trim());
}

/**
 * Setup Amber form event listeners
 */
function setupAmberForm() {
  const bunkerUrlInput = document.getElementById("amber-bunker-url");

  if (bunkerUrlInput) {
    // Validate bunker URL on input
    bunkerUrlInput.addEventListener("input", () => {
      const url = bunkerUrlInput.value.trim();

      if (url && !validateBunkerUrl(url)) {
        bunkerUrlInput.classList.add("border-red-500");
      } else {
        bunkerUrlInput.classList.remove("border-red-500");
      }
    });
  }

  // Check Amber availability and update UI only if elements exist
  const amberForm = document.getElementById("amber-form");
  if (amberForm && !isAmberAvailable()) {
    // Only show error when the form is actually displayed
    console.log("Amber is only available on Android devices");
  }
}

// Initialize form when DOM is ready - but don't auto-run
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => {
    // Only setup if amber form exists
    if (document.getElementById("amber-form")) {
      setupAmberForm();
    }
  });
} else {
  // Only setup if amber form exists
  if (document.getElementById("amber-form")) {
    setupAmberForm();
  }
}

// Export functions to global scope for HTML onclick handlers
window.connectAmber = connectAmber;

// Export AuthAmber object for programmatic access
window.AuthAmber = {
  connect: connectAmber,
  signEvent: signEventWithAmber,
  isAvailable: isAmberAvailable,
  validateBunkerUrl,
  setupForm: setupAmberForm,
};
