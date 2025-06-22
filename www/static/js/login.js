// Global state
let currentAuthMethod = null;
let amberCallbackReceived = false;

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
 * Extension detection and connection
 */
function checkForExtension() {
  const statusEl = document.getElementById("extension-status");
  const connectBtn = document.getElementById("connect-extension");

  if (window.nostr) {
    statusEl.innerHTML =
      '<div class="text-green-200">✅ Nostr extension detected!</div>';
    statusEl.className =
      "p-3 mb-4 bg-green-800 border border-green-600 rounded-lg";
    connectBtn.disabled = false;

    // Log extension capabilities
    console.log("Extension capabilities:", {
      hasGetPublicKey: !!window.nostr.getPublicKey,
      hasSignEvent: !!window.nostr.signEvent,
      hasNip04: !!window.nostr.nip04,
      hasNip44: !!window.nostr.nip44,
    });
  } else {
    statusEl.innerHTML =
      '<div class="text-red-200">❌ No extension found. Please install Alby, nos2x, or another Nostr extension.</div>';
    statusEl.className = "p-3 mb-4 bg-red-800 border border-red-600 rounded-lg";
    connectBtn.disabled = true;
  }
}

/**
 * Connect using browser extension (NIP-07)
 */
async function connectExtension() {
  try {
    showAuthResult("loading", "Requesting access from extension...");

    if (!window.nostr) {
      throw new Error("Nostr extension not found");
    }

    const publicKey = await window.nostr.getPublicKey();

    if (!publicKey || publicKey.length !== 64) {
      throw new Error("Invalid public key received from extension");
    }

    console.log("Extension returned public key:", publicKey);
    showAuthResult("loading", "Creating session with extension signing...");

    const sessionRequest = {
      public_key: publicKey,
      requested_mode: "write",
      signing_method: "browser_extension",
    };

    const response = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(sessionRequest),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      const errorMsg = errorData?.message || `HTTP ${response.status}`;
      throw new Error(`Login failed: ${errorMsg}`);
    }

    const result = await response.json();

    if (!result.success) {
      throw new Error(result.message || "Login failed");
    }

    console.log("Extension login successful:", {
      pubkey: result.session?.public_key,
      mode: result.session?.mode,
      signing_method: result.session?.capabilities?.signing_method,
    });

    showAuthResult("success", "Connected via browser extension!");

    window.nostrExtensionConnected = true;

    setTimeout(() => {
      hideAuthModal();
      if (window.updateNavigation) {
        window.updateNavigation();
      }
      if (result.redirect_url) {
        setTimeout(() => {
          htmx.ajax("GET", "/views/profile.html", "#main-content");
          window.history.pushState({}, "", "/profile");
        }, 500);
      }
    }, 1000);
  } catch (error) {
    console.error("Extension connection error:", error);
    showAuthResult("error", `Extension error: ${error.message}`);
  }
}

/**
 * Amber handling - Implements Nostr Signer protocol
 */
function connectAmber() {
  const bunkerUrl = document.getElementById("amber-bunker-url").value.trim();

  // If no bunker URL provided, try direct Amber connection
  if (!bunkerUrl) {
    connectAmberDirect();
    return;
  }

  // If bunker URL provided, validate and use bunker connection
  if (!bunkerUrl.startsWith("bunker://")) {
    showAuthResult("error", "Invalid bunker URL format");
    return;
  }

  connectAmberBunker(bunkerUrl);
}

/**
 * Connect to Amber directly using Nostr Signer protocol
 */
function connectAmberDirect() {
  showAuthResult("loading", "Opening Amber app...");

  // Generate callback URL for this session
  const callbackUrl = `${window.location.origin}/amber-callback`;

  // Set up callback listener before opening Amber
  setupAmberCallback();

  // Use Amber's get_public_key method with Nostr Signer protocol
  const amberUrl = `nostrsigner:?compressionType=none&returnType=signature&type=get_public_key&callbackUrl=${encodeURIComponent(
    callbackUrl
  )}&appName=${encodeURIComponent("Grain Relay")}`;

  console.log("Opening Amber with URL:", amberUrl);

  try {
    // Attempt to open Amber
    window.location.href = amberUrl;

    // Set timeout in case user doesn't complete the flow
    setTimeout(() => {
      if (!amberCallbackReceived) {
        showAuthResult(
          "error",
          "Amber connection timed out. Make sure Amber is installed and try again."
        );
      }
    }, 30000);
  } catch (error) {
    console.error("Error opening Amber:", error);
    showAuthResult(
      "error",
      "Failed to open Amber app. Please ensure it's installed."
    );
  }
}

/**
 * Connect to Amber using bunker URL (NIP-46)
 */
function connectAmberBunker(bunkerUrl) {
  showAuthResult("loading", "Connecting to Amber bunker...");

  try {
    const url = new URL(bunkerUrl);
    const publicKey = url.hostname;
    const relay = url.searchParams.get("relay");

    if (!publicKey || publicKey.length !== 64) {
      throw new Error("Invalid public key in bunker URL");
    }

    if (!relay) {
      throw new Error("No relay specified in bunker URL");
    }

    console.log("Parsed bunker URL:", { publicKey, relay });

    // Create session with bunker signing method
    createAmberSession(publicKey, "bunker", { bunkerUrl, relay });
  } catch (error) {
    console.error("Error parsing bunker URL:", error);
    showAuthResult("error", "Invalid bunker URL format");
  }
}

/**
 * Set up callback handler for Amber responses
 */
function setupAmberCallback() {
  // Listen for page visibility changes to detect return from Amber
  const handleVisibilityChange = () => {
    if (
      !document.hidden &&
      currentAuthMethod === "amber" &&
      !amberCallbackReceived
    ) {
      // Check URL for callback parameters after a short delay
      setTimeout(checkForAmberCallback, 500);
    }
  };

  document.addEventListener("visibilitychange", handleVisibilityChange);

  // Also check immediately in case we're already on callback page
  setTimeout(checkForAmberCallback, 1000);
}

/**
 * Check current URL for Amber callback parameters
 */
function checkForAmberCallback() {
  const currentUrl = new URL(window.location.href);

  // Check if this is an Amber callback
  if (
    currentUrl.pathname === "/amber-callback" ||
    currentUrl.searchParams.has("event")
  ) {
    handleAmberCallback(currentUrl);
  }
}

/**
 * Handle callback from Amber with public key
 */
function handleAmberCallback(url) {
  try {
    amberCallbackReceived = true;

    const eventParam = url.searchParams.get("event");

    if (!eventParam) {
      throw new Error("No event data received from Amber");
    }

    console.log("Received Amber callback:", eventParam);

    // For get_public_key, the event parameter contains the public key
    let publicKey = eventParam;

    // Handle compressed response (starts with "Signer1")
    if (eventParam.startsWith("Signer1")) {
      try {
        // For compressed responses, we'd need to decompress
        // For now, treat as error since we specify no compression
        throw new Error("Received compressed response unexpectedly");
      } catch (error) {
        console.warn("Failed to handle compressed Amber response:", error);
        throw new Error("Unable to process Amber response");
      }
    }

    // Validate public key
    if (!publicKey || !isValidPublicKey(publicKey)) {
      throw new Error("Invalid public key received from Amber");
    }

    console.log("Amber returned public key:", publicKey);

    // Create session with Amber signing
    createAmberSession(publicKey, "amber");

    // Clean up URL
    window.history.replaceState({}, "", window.location.pathname);
  } catch (error) {
    console.error("Error handling Amber callback:", error);
    showAuthResult("error", `Amber callback error: ${error.message}`);
  }
}

/**
 * Create session with Amber signing method
 */
async function createAmberSession(publicKey, signingMethod, metadata = {}) {
  try {
    showAuthResult("loading", "Creating session with Amber signing...");

    const sessionRequest = {
      public_key: publicKey,
      requested_mode: "write",
      signing_method: signingMethod,
    };

    // Add metadata if provided
    if (Object.keys(metadata).length > 0) {
      sessionRequest.metadata = metadata;
    }

    const response = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(sessionRequest),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      const errorMsg = errorData?.message || `HTTP ${response.status}`;
      throw new Error(`Login failed: ${errorMsg}`);
    }

    const result = await response.json();

    if (!result.success) {
      throw new Error(result.message || "Login failed");
    }

    console.log("Amber login successful:", {
      pubkey: result.session?.public_key,
      mode: result.session?.mode,
      signing_method: result.session?.capabilities?.signing_method,
    });

    showAuthResult("success", "Connected via Amber!");

    // Store Amber connection info
    window.amberConnected = true;
    window.amberSigningMethod = signingMethod;
    if (metadata.bunkerUrl) {
      window.amberBunkerUrl = metadata.bunkerUrl;
    }

    setTimeout(() => {
      hideAuthModal();
      if (window.updateNavigation) {
        window.updateNavigation();
      }
      if (result.redirect_url) {
        setTimeout(() => {
          htmx.ajax("GET", "/views/profile.html", "#main-content");
          window.history.pushState({}, "", "/profile");
        }, 500);
      }
    }, 1000);
  } catch (error) {
    console.error("Amber session creation error:", error);
    showAuthResult("error", `Amber login failed: ${error.message}`);
  }
}

/**
 * Bunker handling
 */
function connectBunker() {
  const bunkerUrl = document.getElementById("bunker-url").value.trim();

  if (!bunkerUrl) {
    showAuthResult("error", "Please enter a bunker URL");
    return;
  }

  if (!bunkerUrl.startsWith("bunker://")) {
    showAuthResult("error", "Invalid bunker URL format");
    return;
  }

  showAuthResult("loading", "Connecting to bunker...");

  // TODO: Implement NIP-46 bunker connection logic
  setTimeout(() => {
    showAuthResult("error", "Bunker integration coming soon!");
  }, 1000);
}

/**
 * Read-only login handling - FIXED API endpoint
 */
function connectReadOnly() {
  const pubkey = document.getElementById("readonly-pubkey").value.trim();

  if (!pubkey) {
    showAuthResult("error", "Please enter a public key");
    return;
  }

  if (!isValidPublicKey(pubkey)) {
    showAuthResult("error", "Invalid public key format");
    return;
  }

  showAuthResult("loading", "Creating read-only session...");

  // Keep npub as-is, let backend handle conversion
  const sessionRequest = {
    public_key: pubkey,
    requested_mode: "read_only",
    signing_method: "none",
  };

  // FIXED: Use correct API endpoint
  fetch("/api/v1/auth/login", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(sessionRequest),
  })
    .then((response) => response.json())
    .then((result) => {
      if (result.success) {
        showAuthResult("success", "Read-only session created!");
        setTimeout(() => {
          hideAuthModal();
          if (window.updateNavigation) {
            window.updateNavigation();
          }
          if (result.redirect_url) {
            setTimeout(() => {
              htmx.ajax("GET", "/views/profile.html", "#main-content");
              window.history.pushState({}, "", "/profile");
            }, 500);
          }
        }, 1000);
      } else {
        showAuthResult("error", result.message || "Login failed");
      }
    })
    .catch((error) => {
      console.error("Read-only login error:", error);
      showAuthResult("error", "Connection failed");
    });
}

/**
 * Private key handling
 */
function connectPrivateKey() {
  const privateKey = document.getElementById("private-key").value.trim();
  const sessionPassword = document.getElementById("session-password").value;

  if (!privateKey || !sessionPassword) {
    showAuthResult("error", "Please fill in all fields");
    return;
  }

  showAuthResult("loading", "Encrypting and storing key...");

  // TODO: Implement private key encryption and storage
  setTimeout(() => {
    showAuthResult("error", "Private key authentication coming soon!");
  }, 1000);
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

/**
 * Sign event using browser extension (NIP-07)
 */
async function signEventWithExtension(event) {
  try {
    if (!window.nostr) {
      throw new Error("Nostr extension not available");
    }

    if (!window.nostrExtensionConnected) {
      throw new Error("Extension not connected - please login first");
    }

    const eventToSign = {
      kind: event.kind,
      created_at: event.created_at || Math.floor(Date.now() / 1000),
      tags: event.tags || [],
      content: event.content || "",
    };

    console.log("Signing event with extension:", eventToSign);

    const signedEvent = await window.nostr.signEvent(eventToSign);

    if (!signedEvent || !signedEvent.id || !signedEvent.sig) {
      throw new Error("Extension returned invalid signed event");
    }

    console.log("Event signed successfully:", signedEvent.id);
    return signedEvent;
  } catch (error) {
    console.error("Extension signing error:", error);
    throw error;
  }
}

/**
 * Check if browser extension is available and connected
 */
function isExtensionAvailable() {
  return !!(window.nostr && window.nostrExtensionConnected);
}

/**
 * Check if Amber is available and connected
 */
function isAmberAvailable() {
  return !!window.amberConnected;
}

/**
 * Get public key from extension
 */
async function getExtensionPublicKey() {
  if (!window.nostr) {
    throw new Error("Extension not available");
  }
  return await window.nostr.getPublicKey();
}

// Expose functions globally
window.showAuthModal = showAuthModal;
window.hideAuthModal = hideAuthModal;
window.signEventWithExtension = signEventWithExtension;
window.isExtensionAvailable = isExtensionAvailable;
window.isAmberAvailable = isAmberAvailable;
window.getExtensionPublicKey = getExtensionPublicKey;
