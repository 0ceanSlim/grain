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
 * Check if Amber is available and connected
 */
function isAmberAvailable() {
  return !!window.amberConnected;
}
