/**
 * Dropdown functionality utilities
 * Handles user dropdown menu interactions
 */

// Global dropdown functions
window.toggleUserDropdown = function () {
  const dropdown = document.getElementById("user-dropdown-menu");
  if (!dropdown) {
    console.error("User dropdown menu not found");
    return;
  }

  const isHidden = dropdown.classList.contains("hidden");

  if (isHidden) {
    dropdown.classList.remove("hidden");
    // Close dropdown when clicking outside
    document.addEventListener("click", window.handleClickOutside);
  } else {
    dropdown.classList.add("hidden");
    document.removeEventListener("click", window.handleClickOutside);
  }
};

window.closeUserDropdown = function () {
  const dropdown = document.getElementById("user-dropdown-menu");
  if (dropdown) {
    dropdown.classList.add("hidden");
    document.removeEventListener("click", window.handleClickOutside);
  }
};

window.handleClickOutside = function (event) {
  const dropdown = document.getElementById("user-dropdown-menu");
  const button = document.getElementById("user-menu-button");

  if (
    dropdown &&
    !dropdown.contains(event.target) &&
    button &&
    !button.contains(event.target)
  ) {
    window.closeUserDropdown();
  }
};

// Clean up event listeners on page unload
window.addEventListener("beforeunload", function () {
  document.removeEventListener("click", window.handleClickOutside);
});
