/**
 * Authentication modal and method selection manager
 * Handles the main auth flow and routing to specific auth methods
 */

let currentAuthMethod = null;

/**
 * Show authentication modal
 */
function showAuthModal() {
  ui.show("auth-modal");
  resetAuthModal();
}

/**
 * Hide authentication modal
 */
function hideAuthModal() {
  ui.hide("auth-modal");
  resetAuthModal();
}

/**
 * Reset modal to initial state
 */
function resetAuthModal() {
  // Hide all auth method forms
  const authForms = [
    "extension-form",
    "amber-form",
    "bunker-form",
    "readonly-form",
    "privkey-form",
  ];

  authForms.forEach((id) => ui.hide(id));

  // Show method selection screen
  ui.show("auth-method-selection");
  ui.show("close-button");

  // Clear all form inputs
  AuthBase.clearAuthInputs([
    "bunker-url",
    "amber-bunker-url",
    "readonly-pubkey",
    "private-key",
    "session-password",
  ]);

  // Clear any result messages
  ui.setHtml("auth-result", "");

  currentAuthMethod = null;
}

/**
 * Select authentication method and show appropriate form
 * @param {string} method - Auth method: 'extension', 'amber', 'bunker', 'readonly', 'privkey'
 */
function selectAuthMethod(method) {
  currentAuthMethod = method;

  // Hide method selection
  ui.hide("auth-method-selection");
  ui.hide("close-button");

  // Show selected method form
  ui.show(`${method}-form`);

  // Handle method-specific initialization
  switch (method) {
    case "extension":
      if (window.AuthExtension && window.AuthExtension.checkExtension) {
        window.AuthExtension.checkExtension();
      }
      break;
    case "amber":
      // Amber-specific initialization if needed
      break;
    case "bunker":
      // Bunker-specific initialization if needed
      break;
    case "readonly":
      // Read-only specific initialization if needed
      break;
    case "privkey":
      // Private key specific initialization if needed
      break;
    default:
      console.warn(`Unknown auth method: ${method}`);
  }
}

/**
 * Go back to method selection from current form
 */
function goBack() {
  resetAuthModal();
}

/**
 * Get current authentication method
 * @returns {string|null} Current method or null if none selected
 */
function getCurrentAuthMethod() {
  return currentAuthMethod;
}

// Export functions to global scope for HTML onclick handlers
window.showAuthModal = showAuthModal;
window.hideAuthModal = hideAuthModal;
window.selectAuthMethod = selectAuthMethod;
window.goBack = goBack;

// Export AuthManager object for programmatic access
window.AuthManager = {
  showModal: showAuthModal,
  hideModal: hideAuthModal,
  selectMethod: selectAuthMethod,
  goBack,
  getCurrentMethod: getCurrentAuthMethod,
  resetModal: resetAuthModal,
};
