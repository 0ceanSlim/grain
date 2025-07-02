/**
 * Amber NIP-55 Implementation
 * Based on NIP-55 spec for web applications with mobile browser support
 */

let amberCallbackReceived = false;

window.isAmberAvailable = isAmberAvailable;

/**
 * Debug function to test Amber protocol support
 */
function testAmberProtocol() {
  console.log("Testing Amber protocol support...");

  const testUrl = "nostrsigner:?type=test";

  try {
    const anchor = document.createElement("a");
    anchor.href = testUrl;
    anchor.target = "_blank";
    anchor.style.display = "none";
    document.body.appendChild(anchor);

    anchor.click();

    setTimeout(() => {
      if (document.body.contains(anchor)) {
        document.body.removeChild(anchor);
      }
    }, 100);

    console.log("Test protocol URL triggered successfully");
    return true;
  } catch (error) {
    console.error("Test protocol failed:", error);
    return false;
  }
}

/**
 * Check if Amber is available and connected
 */
function isAmberAvailable() {
  return !!window.amberConnected;
}

/**
 * Process the actual callback data
 */
function handleAmberCallbackData(data) {
  try {
    // If there's an error in the stored data
    if (data.error) {
      throw new Error(data.error);
    }

    // Session was already created by the callback handler
    // Just show success and update UI like extension login does
    console.log("Amber login completed successfully");

    showAuthResult("success", "Connected via Amber!");

    // Store Amber connection info
    window.amberConnected = true;
    window.amberSigningMethod = "amber";

    // Hide modal and update navigation like extension login
    setTimeout(() => {
      hideAuthModal();
      if (window.updateNavigation) {
        window.updateNavigation();
      }
      // The session is already created, just refresh the page state
      // No need to manually navigate - let the app detect the login state
    }, 1000);
  } catch (error) {
    console.error("Error processing Amber callback data:", error);
    showAuthResult("error", `Amber login failed: ${error.message}`);
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

    handleAmberCallbackData({ event: eventParam });
  } catch (error) {
    console.error("Error handling Amber callback:", error);
    showAuthResult("error", `Amber callback error: ${error.message}`);
  }
}

/**
 * Check if we're on the callback URL or if callback data is available
 */
function checkForAmberCallback() {
  const currentUrl = new URL(window.location.href);

  // Check if this is the amber-callback page
  if (currentUrl.pathname === "/api/v1/auth/amber-callback") {
    handleAmberCallback(currentUrl);
    return;
  }

  // Check if we have the event parameter in current URL
  if (currentUrl.searchParams.has("event")) {
    handleAmberCallback(currentUrl);
    return;
  }

  // Check if data was stored in localStorage by the callback page
  const amberResult = localStorage.getItem("amber_callback_result");
  if (amberResult) {
    try {
      const data = JSON.parse(amberResult);
      localStorage.removeItem("amber_callback_result");
      handleAmberCallbackData(data);
    } catch (error) {
      console.error("Failed to parse stored Amber result:", error);
    }
  }
}

/**
 * Set up proper callback listener using window focus and URL checking
 */
function setupAmberCallbackListener() {
  // Listen for when user returns to the page
  const handleVisibilityChange = () => {
    if (!document.hidden && !amberCallbackReceived) {
      // Check if we're on the callback URL
      setTimeout(checkForAmberCallback, 500);
    }
  };

  const handleFocus = () => {
    if (!amberCallbackReceived) {
      setTimeout(checkForAmberCallback, 500);
    }
  };

  // Add multiple listeners to catch the return
  document.addEventListener("visibilitychange", handleVisibilityChange);
  window.addEventListener("focus", handleFocus);

  // Also check immediately
  setTimeout(checkForAmberCallback, 1000);

  // Clean up listeners after timeout
  setTimeout(() => {
    document.removeEventListener("visibilitychange", handleVisibilityChange);
    window.removeEventListener("focus", handleFocus);
  }, 65000);
}

/**
 * Connect using bunker URL (for future NIP-46 implementation)
 */
function connectAmberBunker(bunkerUrl) {
  showAuthResult("loading", "Bunker connections not implemented yet");
  console.log("Future NIP-46 bunker connection:", bunkerUrl);

  setTimeout(() => {
    showAuthResult(
      "error",
      "Bunker connections coming soon! Use direct connection for now."
    );
  }, 2000);
}

/**
 * Connect to Amber directly using NIP-55 protocol
 */
function connectAmberDirect() {
  showAuthResult("loading", "Opening Amber app...");

  // Set up callback listener BEFORE opening Amber
  setupAmberCallbackListener();

  // Generate proper callback URL that your server can handle
  const callbackUrl = `${window.location.origin}/api/v1/auth/amber-callback?event=`;

  // Use proper NIP-55 nostrsigner URL format
  const amberUrl = `nostrsigner:?compressionType=none&returnType=signature&type=get_public_key&callbackUrl=${encodeURIComponent(
    callbackUrl
  )}&appName=${encodeURIComponent("Grain Relay")}`;

  console.log("Opening Amber with URL:", amberUrl);

  try {
    // Try multiple approaches for opening the nostrsigner protocol
    let protocolOpened = false;

    // Method 1: Create anchor element and click it (most reliable on mobile)
    try {
      const anchor = document.createElement("a");
      anchor.href = amberUrl;
      anchor.target = "_blank";
      anchor.style.display = "none";
      document.body.appendChild(anchor);

      // Trigger click to open Amber
      anchor.click();
      protocolOpened = true;

      // Clean up anchor element
      setTimeout(() => {
        if (document.body.contains(anchor)) {
          document.body.removeChild(anchor);
        }
      }, 100);

      console.log("Amber protocol opened via anchor click");
    } catch (anchorError) {
      console.warn("Anchor method failed:", anchorError);
    }

    // Method 2: Fallback to window.location.href if anchor didn't work
    if (!protocolOpened) {
      try {
        window.location.href = amberUrl;
        protocolOpened = true;
        console.log("Amber protocol opened via window.location.href");
      } catch (locationError) {
        console.warn("Window location method failed:", locationError);
      }
    }

    // Method 3: Last resort - try window.open
    if (!protocolOpened) {
      try {
        const newWindow = window.open(amberUrl, "_blank");
        if (newWindow) {
          newWindow.close(); // Close immediately, we just want to trigger the protocol
          protocolOpened = true;
          console.log("Amber protocol opened via window.open");
        }
      } catch (openError) {
        console.warn("Window open method failed:", openError);
      }
    }

    if (!protocolOpened) {
      throw new Error("Unable to open Amber protocol - no method worked");
    }

    // Show additional guidance for mobile users
    showAuthResult(
      "loading",
      "Opening Amber app... If nothing happens, make sure Amber is installed and try again."
    );

    // Set timeout in case user doesn't complete the flow
    setTimeout(() => {
      if (!amberCallbackReceived) {
        showAuthResult(
          "error",
          "Amber connection timed out. Make sure Amber is installed and try again. If the app opened but didn't return, check your Amber app permissions."
        );
      }
    }, 60000); // 60 seconds timeout
  } catch (error) {
    console.error("Error opening Amber:", error);
    showAuthResult(
      "error",
      "Failed to open Amber app. Please ensure Amber is installed and your browser supports the nostrsigner protocol."
    );
  }
}

/**
 * Connect to Amber using proper NIP-55 protocol
 */
function connectAmber() {
  const bunkerUrl = document.getElementById("amber-bunker-url").value.trim();

  // If bunker URL provided, use bunker connection (NIP-46 - for later)
  if (bunkerUrl) {
    if (!bunkerUrl.startsWith("bunker://")) {
      showAuthResult("error", "Invalid bunker URL format");
      return;
    }
    connectAmberBunker(bunkerUrl);
    return;
  }

  // Use direct Amber connection (NIP-55)
  connectAmberDirect();
}

// Expose test function globally for debugging
window.testAmberProtocol = testAmberProtocol;
