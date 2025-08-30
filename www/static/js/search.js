/**
 * Enhanced Search Bar functionality with help system
 * Handles parsing of nostr identifiers and routing to appropriate endpoints
 */

console.log("[SEARCH] Enhanced search functionality loaded");

let searchTimeout;

// Initialize search functionality when DOM is ready
function initializeSearch() {
  const searchInput = document.getElementById("search-input");
  const searchButton = document.getElementById("search-button");
  const searchResults = document.getElementById("search-results");
  const searchHelpButton = document.getElementById("search-help-button");
  const searchHelpPanel = document.getElementById("search-help-panel");
  const closeHelpButton = document.getElementById("close-help");

  if (!searchInput || !searchButton) {
    console.log("[SEARCH] Search elements not found, will retry on next load");
    return;
  }

  console.log("[SEARCH] Search elements found, initializing");

  // Handle search on Enter key
  searchInput.addEventListener("keypress", function (e) {
    if (e.key === "Enter") {
      e.preventDefault();
      performSearch();
    }
  });

  // Handle search button click
  searchButton.addEventListener("click", function (e) {
    e.preventDefault();
    performSearch();
  });

  // Handle input changes for live suggestions (debounced)
  searchInput.addEventListener("input", function (e) {
    clearTimeout(searchTimeout);
    const query = e.target.value.trim();

    if (query.length < 4) {
      hideSearchResults();
      return;
    }

    searchTimeout = setTimeout(() => {
      showSearchSuggestions(query);
    }, 300);
  });

  // Help system event handlers
  if (searchHelpButton && searchHelpPanel) {
    searchHelpButton.addEventListener("click", function (e) {
      e.preventDefault();
      e.stopPropagation();
      toggleHelpPanel();
    });

    if (closeHelpButton) {
      closeHelpButton.addEventListener("click", function (e) {
        e.preventDefault();
        hideHelpPanel();
      });
    }
  }

  // Hide panels when clicking outside
  document.addEventListener("click", function (e) {
    if (!e.target.closest("#search-container")) {
      hideSearchResults();
      hideHelpPanel();
    }
  });

  // Hide help panel when starting to type
  if (searchInput && searchHelpPanel) {
    searchInput.addEventListener("input", function () {
      if (searchInput.value.trim()) {
        hideHelpPanel();
      }
    });
  }
}

// Show/hide help panel
function toggleHelpPanel() {
  const helpPanel = document.getElementById("search-help-panel");
  const searchResults = document.getElementById("search-results");

  if (!helpPanel) return;

  if (helpPanel.classList.contains("hidden")) {
    // Hide search results first
    hideSearchResults();

    // Show help panel
    helpPanel.classList.remove("hidden");
    console.log("[SEARCH] Help panel shown");
  } else {
    hideHelpPanel();
  }
}

function hideHelpPanel() {
  const helpPanel = document.getElementById("search-help-panel");
  if (helpPanel) {
    helpPanel.classList.add("hidden");
  }
}

// Perform search and navigate to appropriate page
async function performSearch() {
  const searchInput = document.getElementById("search-input");
  if (!searchInput) return;

  const query = searchInput.value.trim();
  if (!query) return;

  console.log("[SEARCH] Performing search for:", query);

  try {
    const { type, identifier } = parseNostrIdentifier(query);

    let targetPath;
    let targetView;
    let finalIdentifier = identifier;

    // For complex bech32 entities, decode them first
    if (type === "nprofile" || type === "nevent" || type === "naddr") {
      console.log("[SEARCH] Decoding complex bech32 entity:", type, identifier);

      try {
        // Use POST for long entities to avoid URL length limits
        const response = await fetch(`/api/v1/keys/decode/nip19/`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            entity: identifier,
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            "[SEARCH] Decode API error:",
            response.status,
            errorText
          );
          throw new Error(`Failed to decode ${type}: ${response.status}`);
        }

        const decoded = await response.json();
        console.log("[SEARCH] Decoded entity successfully:", decoded);

        // Use the decoded data for navigation
        if (decoded.data) {
          finalIdentifier = decoded.data;
          console.log("[SEARCH] Using decoded data:", finalIdentifier);
        } else {
          console.warn(
            "[SEARCH] No data field in decoded response, using original"
          );
          finalIdentifier = identifier;
        }
      } catch (decodeError) {
        console.error("[SEARCH] Failed to decode complex entity:", decodeError);
        showSearchError(`Failed to decode ${type}: ${decodeError.message}`);
        return; // Stop here instead of falling back
      }
    }

    // Determine target based on type
    switch (type) {
      case "npub":
      case "nprofile":
        targetPath = `/p/${finalIdentifier}`;
        targetView = "/views/components/profile-page.html";
        break;

      case "note":
      case "nevent":
      case "naddr":
      case "eventid":
        targetPath = `/e/${finalIdentifier}`;
        targetView = "/views/components/event-page.html";
        break;

      default:
        throw new Error("Unrecognized identifier type");
    }

    console.log("[SEARCH] Final navigation details:", {
      type,
      originalIdentifier: identifier,
      finalIdentifier,
      targetPath,
    });

    // Navigate using HTMX and update URL
    if (typeof htmx !== "undefined") {
      htmx.ajax("GET", targetView, { target: "#main-content" });
      window.history.pushState({}, "", targetPath);

      // Clear search input and hide all panels
      searchInput.value = "";
      hideSearchResults();
      hideHelpPanel();

      console.log("[SEARCH] Navigation completed to:", targetPath);
    } else {
      console.error("[SEARCH] HTMX not available for navigation");
    }
  } catch (error) {
    console.error("[SEARCH] Search failed:", error);
    showSearchError(
      "Invalid identifier format. Please check the help for supported formats."
    );
  }
}

// Parse nostr identifiers according to NIP-19
function parseNostrIdentifier(input) {
  const trimmed = input.trim();
  const lower = trimmed.toLowerCase();

  // Check for npub (bech32 public key)
  if (lower.startsWith("npub1")) {
    return { type: "npub", identifier: trimmed };
  }

  // Check for note (bech32 event ID)
  if (lower.startsWith("note1")) {
    return { type: "note", identifier: trimmed };
  }

  // Check for nprofile (bech32 profile with metadata)
  if (lower.startsWith("nprofile1")) {
    return { type: "nprofile", identifier: trimmed };
  }

  // Check for nevent (bech32 event with metadata)
  if (lower.startsWith("nevent1")) {
    return { type: "nevent", identifier: trimmed };
  }

  // Check for naddr (bech32 replaceable event coordinate)
  if (lower.startsWith("naddr1")) {
    return { type: "naddr", identifier: trimmed };
  }

  // Check for hex (64 characters) - always treat as event ID
  if (/^[0-9a-f]{64}$/i.test(lower)) {
    return { type: "eventid", identifier: lower };
  }

  throw new Error("Unrecognized identifier format");
}

// Show search suggestions with enhanced format detection
function showSearchSuggestions(query) {
  const searchResults = document.getElementById("search-results");
  if (!searchResults) return;

  try {
    const { type } = parseNostrIdentifier(query);

    let description;
    let icon;

    switch (type) {
      case "npub":
        description = "Public key (profile)";
        icon = "👤";
        break;
      case "note":
        description = "Event ID (note)";
        icon = "📝";
        break;
      case "nprofile":
        description = "Profile with metadata";
        icon = "👤+";
        break;
      case "nevent":
        description = "Event with metadata";
        icon = "📝+";
        break;
      case "naddr":
        description = "Replaceable event coordinate";
        icon = "🔗";
        break;
      case "eventid":
        description = "Hex event ID";
        icon = "📝";
        break;
      default:
        description = "Unknown format";
        icon = "❓";
    }

    searchResults.innerHTML = `
      <div class="p-3 text-sm text-gray-300">
        <div class="flex items-center space-x-2">
          <span class="text-green-400">✓</span>
          <span>${icon}</span>
          <span>Valid ${description}</span>
        </div>
        <div class="mt-1 text-xs text-gray-400">
          Press Enter to search
        </div>
      </div>
    `;

    searchResults.classList.remove("hidden");
  } catch (error) {
    // Hide results for invalid formats
    hideSearchResults();
  }
}

// Show search error
function showSearchError(message) {
  const searchResults = document.getElementById("search-results");
  if (!searchResults) return;

  searchResults.innerHTML = `
    <div class="p-3 text-sm text-red-400">
      <div class="flex items-center space-x-2">
        <span>⚠</span>
        <span>${message}</span>
      </div>
      <div class="mt-1 text-xs text-gray-400">
        Click the ? button for help
      </div>
    </div>
  `;

  searchResults.classList.remove("hidden");

  // Hide error after 5 seconds
  setTimeout(hideSearchResults, 5000);
}

// Hide search results
function hideSearchResults() {
  const searchResults = document.getElementById("search-results");
  if (searchResults) {
    searchResults.classList.add("hidden");
  }
}

// Initialize search when DOM is ready or when header is loaded
function safeInitializeSearch() {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initializeSearch);
  } else {
    initializeSearch();
  }
}

// Also try to initialize when header is updated
document.addEventListener("htmx:afterSwap", function (e) {
  // Check if the swapped content contains search elements
  if (e.detail.target.querySelector("#search-input")) {
    console.log("[SEARCH] Header swapped, reinitializing search");
    setTimeout(initializeSearch, 100);
  }
});

// Initialize immediately
safeInitializeSearch();

// Export for global access
window.initializeSearch = initializeSearch;
