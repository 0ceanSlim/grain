/**
 * NIP-46 Bunker Authentication
 * Handles login via remote signing bunkers using Nostr Connect protocol
 * TODO: Implement proper NIP-46 protocol integration
 */

/**
 * Connect via NIP-46 bunker
 */
async function connectBunker() {
  const bunkerUrlInput = document.getElementById("bunker-url");

  if (!bunkerUrlInput) {
    AuthBase.showResult("error", "Bunker form not found");
    return;
  }

  const bunkerUrl = bunkerUrlInput.value.trim();

  // Validate bunker URL
  if (!bunkerUrl) {
    AuthBase.showResult("error", "Enter a bunker URL");
    return;
  }

  if (!validateBunkerUrl(bunkerUrl)) {
    AuthBase.showResult("error", "Invalid bunker URL format");
    return;
  }

  AuthBase.showResult("loading", "Connecting to bunker...");

  try {
    await establishBunkerConnection(bunkerUrl);
  } catch (error) {
    AuthBase.showResult("error", `Bunker connection failed: ${error.message}`);
  }
}

/**
 * Establish connection to NIP-46 bunker
 * @param {string} bunkerUrl - Bunker URL in format bunker://pubkey?relay=wss://...
 */
async function establishBunkerConnection(bunkerUrl) {
  // Parse bunker URL
  const parsedUrl = parseBunkerUrl(bunkerUrl);
  if (!parsedUrl) {
    throw new Error("Could not parse bunker URL");
  }

  const { pubkey, relay } = parsedUrl;

  // TODO: Implement NIP-46 connection flow
  // This should:
  // 1. Connect to the specified relay
  // 2. Generate a local keypair for this session
  // 3. Send a connection request to the bunker pubkey
  // 4. Handle the connection response
  // 5. Send auth challenge if required
  // 6. Create session with bunker signing method

  throw new Error("NIP-46 bunker connection not yet implemented");
}

/**
 * Parse bunker URL into components
 * @param {string} url - Bunker URL
 * @returns {Object|null} Parsed components or null if invalid
 */
function parseBunkerUrl(url) {
  try {
    // Expected format: bunker://pubkey?relay=wss://relay.url&secret=optional
    const urlObj = new URL(url);

    if (urlObj.protocol !== "bunker:") {
      return null;
    }

    const pubkey = urlObj.hostname || urlObj.pathname.replace("//", "");
    const relay = urlObj.searchParams.get("relay");
    const secret = urlObj.searchParams.get("secret");

    if (!pubkey || !relay || pubkey.length !== 64) {
      return null;
    }

    return { pubkey, relay, secret };
  } catch (error) {
    return null;
  }
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

  const parsed = parseBunkerUrl(url.trim());
  return parsed !== null;
}

/**
 * Sign event using NIP-46 bunker
 * @param {Object} event - Event to sign
 * @returns {Promise<Object>} Signed event
 */
async function signEventWithBunker(event) {
  // TODO: Implement NIP-46 event signing
  // This should:
  // 1. Create signing request event (kind 24133)
  // 2. Send to bunker via established relay connection
  // 3. Wait for signed response (kind 24133)
  // 4. Extract and return signed event

  throw new Error("NIP-46 bunker event signing not yet implemented");
}

/**
 * Send request to bunker and wait for response
 * @param {string} method - NIP-46 method name
 * @param {Array} params - Method parameters
 * @returns {Promise<any>} Response from bunker
 */
async function sendBunkerRequest(method, params = []) {
  // TODO: Implement NIP-46 request/response flow
  // This should:
  // 1. Create request event with method and params
  // 2. Send to bunker via relay
  // 3. Wait for response event
  // 4. Parse and return response

  throw new Error("NIP-46 bunker requests not yet implemented");
}

/**
 * Disconnect from bunker
 */
async function disconnectBunker() {
  // TODO: Implement proper bunker disconnection
  // This should clean up relay connections and clear session data

  console.log("Bunker disconnection not yet implemented");
}

/**
 * Setup bunker form event listeners
 */
function setupBunkerForm() {
  const bunkerUrlInput = document.getElementById("bunker-url");
  const connectBtn = document.getElementById("connect-bunker");

  if (bunkerUrlInput) {
    // Validate bunker URL on input
    bunkerUrlInput.addEventListener("input", () => {
      const url = bunkerUrlInput.value.trim();

      if (url && !validateBunkerUrl(url)) {
        bunkerUrlInput.classList.add("border-red-500");
        if (connectBtn) connectBtn.disabled = true;
      } else {
        bunkerUrlInput.classList.remove("border-red-500");
        if (connectBtn) connectBtn.disabled = false;
      }
    });

    // Add enter key support
    bunkerUrlInput.addEventListener("keypress", (e) => {
      if (e.key === "Enter" && validateBunkerUrl(bunkerUrlInput.value.trim())) {
        connectBunker();
      }
    });
  }
}

/**
 * Check if WebSocket is supported for relay connections
 * @returns {boolean} True if WebSocket is available
 */
function isWebSocketSupported() {
  return typeof WebSocket !== "undefined";
}

// Initialize form when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", setupBunkerForm);
} else {
  setupBunkerForm();
}

// Export functions to global scope for HTML onclick handlers
window.connectBunker = connectBunker;

// Export AuthBunker object for programmatic access
window.AuthBunker = {
  connect: connectBunker,
  signEvent: signEventWithBunker,
  sendRequest: sendBunkerRequest,
  disconnect: disconnectBunker,
  parseBunkerUrl,
  validateBunkerUrl,
  setupForm: setupBunkerForm,
  isWebSocketSupported,
};
