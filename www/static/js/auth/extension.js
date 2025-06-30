/**
 * Browser Extension Authentication (NIP-07)
 * Handles login via Nostr browser extensions like Alby, nos2x, Flamingo
 */

/**
 * Check if Nostr extension is available and update UI
 */
function checkExtension() {
    const statusEl = document.getElementById("extension-status");
    const connectBtn = document.getElementById("connect-extension");
  
    if (!statusEl || !connectBtn) {
      console.warn("Extension status elements not found");
      return;
    }
  
    if (window.nostr) {
      statusEl.innerHTML = '<div class="text-green-200">✅ Extension detected!</div>';
      statusEl.className = "p-3 mb-4 bg-green-800 border border-green-600 rounded-lg";
      connectBtn.disabled = false;
    } else {
      statusEl.innerHTML = '<div class="text-red-200">❌ No extension found</div>';
      statusEl.className = "p-3 mb-4 bg-red-800 border border-red-600 rounded-lg";
      connectBtn.disabled = true;
    }
  }
  
  /**
   * Connect via browser extension
   */
  async function connectExtension() {
    if (!window.nostr) {
      AuthBase.showResult("error", "Extension not available");
      return;
    }
  
    AuthBase.showResult("loading", "Connecting...");
  
    try {
      // Get public key from extension
      const publicKey = await window.nostr.getPublicKey();
      
      if (!publicKey) {
        throw new Error("Failed to get public key from extension");
      }
  
      // Create login data
      const loginData = AuthBase.createLoginData(
        publicKey,
        "browser_extension", 
        "write"
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
   * Sign event using browser extension
   * @param {Object} event - Event object to sign
   * @returns {Promise<Object>} Signed event
   */
  async function signEventWithExtension(event) {
    if (!window.nostr) {
      throw new Error("Extension not available");
    }
  
    if (!event) {
      throw new Error("No event provided for signing");
    }
  
    try {
      return await window.nostr.signEvent({
        kind: event.kind,
        created_at: event.created_at || Math.floor(Date.now() / 1000),
        tags: event.tags || [],
        content: event.content || "",
      });
    } catch (error) {
      throw new Error(`Extension signing failed: ${error.message}`);
    }
  }
  
  /**
   * Check if extension supports specific NIP-07 methods
   * @returns {Object} Object with method availability 
   */
  function getExtensionCapabilities() {
    if (!window.nostr) {
      return {};
    }
  
    return {
      getPublicKey: typeof window.nostr.getPublicKey === 'function',
      signEvent: typeof window.nostr.signEvent === 'function', 
      getRelays: typeof window.nostr.getRelays === 'function',
      nip04: {
        encrypt: typeof window.nostr.nip04?.encrypt === 'function',
        decrypt: typeof window.nostr.nip04?.decrypt === 'function'
      }
    };
  }
  
  // Export functions to global scope for HTML onclick handlers
  window.connectExtension = connectExtension;
  window.signEventWithExtension = signEventWithExtension;
  
  // Export AuthExtension object for programmatic access
  window.AuthExtension = {
    checkExtension,
    connect: connectExtension,
    signEvent: signEventWithExtension,
    getCapabilities: getExtensionCapabilities
  };