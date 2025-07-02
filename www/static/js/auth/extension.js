/**
 * Extension detection and connection
 */

window.signEventWithExtension = signEventWithExtension;
window.isExtensionAvailable = isExtensionAvailable;
window.getExtensionPublicKey = getExtensionPublicKey;

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
 * Get public key from extension
 */
async function getExtensionPublicKey() {
  if (!window.nostr) {
    throw new Error("Extension not available");
  }
  return await window.nostr.getPublicKey();
}
