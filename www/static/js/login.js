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

// NIP-07 Extension handling
function checkForExtension() {
  const statusEl = document.getElementById("extension-status");
  const connectBtn = document.getElementById("connect-extension");

  if (window.nostr) {
    statusEl.innerHTML =
      '<div class="text-green-200">✅ Extension detected!</div>';
    statusEl.className =
      "p-3 mb-4 bg-green-800 border border-green-600 rounded-lg";
    connectBtn.disabled = false;

    // Log detected extension capabilities
    console.log("Nostr extension detected:", {
      hasGetPublicKey: typeof window.nostr.getPublicKey === "function",
      hasSignEvent: typeof window.nostr.signEvent === "function",
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
 * Gets public key from extension and creates session with browser extension signing
 */
async function connectExtension() {
  try {
    showAuthResult("loading", "Requesting access from extension...");

    // Check if extension is still available
    if (!window.nostr) {
      throw new Error("Nostr extension not found");
    }

    // Get public key from extension using NIP-07
    const publicKey = await window.nostr.getPublicKey();

    if (!publicKey || publicKey.length !== 64) {
      throw new Error("Invalid public key received from extension");
    }

    console.log("Extension returned public key:", publicKey);
    showAuthResult("loading", "Creating session with extension signing...");

    // Create session with browser extension signing method
    const sessionRequest = {
      public_key: publicKey,
      requested_mode: "write", // Extension allows signing
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

    // Store extension capability for future use
    window.nostrExtensionConnected = true;

    // Close modal and update UI
    setTimeout(() => {
      hideAuthModal();

      // Trigger navigation update
      if (window.updateNavigation) {
        window.updateNavigation();
      }

      // Navigate to profile if redirect URL provided
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
 * Sign event using browser extension (NIP-07)
 * This function can be called from anywhere in the app when an event needs signing
 */
async function signEventWithExtension(event) {
  try {
    if (!window.nostr) {
      throw new Error("Nostr extension not available");
    }

    if (!window.nostrExtensionConnected) {
      throw new Error("Extension not connected - please login first");
    }

    // Validate event structure
    if (!event || typeof event !== "object") {
      throw new Error("Invalid event object");
    }

    // Ensure required fields are present
    const eventToSign = {
      kind: event.kind,
      created_at: event.created_at || Math.floor(Date.now() / 1000),
      tags: event.tags || [],
      content: event.content || "",
    };

    console.log("Signing event with extension:", eventToSign);

    // Sign using NIP-07
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
 * Get public key from extension (for verification or display)
 */
async function getExtensionPublicKey() {
  if (!window.nostr) {
    throw new Error("Extension not available");
  }

  return await window.nostr.getPublicKey();
}

// Read-only login handling
function connectReadOnly() {
  const pubkey = document.getElementById("readonly-pubkey").value.trim();

  if (!pubkey) {
    showAuthResult("error", "Please enter a public key");
    return;
  }

  // Validate pubkey format (64 hex chars or npub)
  if (!isValidPublicKey(pubkey)) {
    showAuthResult("error", "Invalid public key format");
    return;
  }

  showAuthResult("loading", "Creating read-only session...");

  // Convert npub to hex if needed
  const hexPubkey = pubkey.startsWith("npub") ? npubToHex(pubkey) : pubkey;

  // Create read-only session
  const sessionRequest = {
    public_key: hexPubkey,
    requested_mode: "read_only",
    signing_method: "none",
  };

  fetch("/api/v1/login", {
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

// Amber handling (placeholder)
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

  // TODO: Implement NIP-46 Amber connection logic
  setTimeout(() => {
    showAuthResult("error", "Amber integration coming soon!");
  }, 1000);
}

// Private key handling (placeholder)
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

// Utility functions
function isValidPublicKey(pubkey) {
  if (pubkey.startsWith("npub")) {
    return pubkey.length === 63; // npub1 + 58 chars
  }
  return /^[0-9a-fA-F]{64}$/.test(pubkey);
}

function npubToHex(npub) {
  // Simple implementation - in production you'd want to use a proper bech32 library
  // This is a placeholder that assumes valid input
  try {
    // You would implement proper bech32 decoding here
    // For now, return as-is and let backend handle conversion
    return npub;
  } catch (error) {
    throw new Error("Invalid npub format");
  }
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

// Expose functions globally for modal triggering and event signing
window.showAuthModal = showAuthModal;
window.hideAuthModal = hideAuthModal;
window.signEventWithExtension = signEventWithExtension;
window.isExtensionAvailable = isExtensionAvailable;
window.getExtensionPublicKey = getExtensionPublicKey;
