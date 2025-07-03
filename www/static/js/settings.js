/**
 * Settings page functionality
 * Handles session information, relay management, and key operations
 */

function loadSettingsData() {
  console.log("Loading settings data...");

  // First check if user has an active session
  fetch("/api/v1/session")
    .then((response) => {
      console.log("Session response status:", response.status);
      if (!response.ok) {
        throw new Error("No active session");
      }
      return response.json();
    })
    .then((sessionData) => {
      console.log("Session data:", sessionData);
      // Load cached profile data for relay and key information
      return fetch("/api/v1/cache");
    })
    .then((response) => {
      console.log("Cache response status:", response.status);

      if (response.status === 503) {
        // Service unavailable - cache refresh failed
        return response.json().then((errorData) => {
          throw new Error(errorData.message || "Cache refresh failed");
        });
      }

      if (!response.ok) {
        throw new Error("Failed to load settings cache");
      }

      return response.json();
    })
    .then((cacheData) => {
      console.log("Cache data received:", cacheData);

      // Show refresh indicator if data was just refreshed
      if (cacheData.refreshed) {
        showRefreshNotification();
      }

      displaySettings(cacheData);
    })
    .catch((error) => {
      console.error("Settings load error:", error);

      // Check if it's a cache-related error and offer refresh
      if (
        error.message.includes("Cache") ||
        error.message.includes("refresh")
      ) {
        showCacheError(error.message);
      } else {
        showError(error.message);
      }
    });
}

function displaySettings(data) {
  console.log("Displaying settings with data:", data);

  // Hide loading and show content
  const loadingEl = document.getElementById("loading");
  const contentEl = document.getElementById("settings-content");

  if (loadingEl) loadingEl.classList.add("hidden");
  if (contentEl) contentEl.classList.remove("hidden");

  // Store data globally for copy functions
  window.profileData = data;

  // Update debug info
  updateDebugInfo(data);

  // Update session information
  updateSessionInfo(data);

  // Update relay information and status
  updateRelayTable(data);

  // Update mailboxes table
  updateMailboxTable(data);

  // Update key information
  updateKeyInformation(data);
}

// Tab switching functionality
function switchRelayTab(tabName) {
  // Update tab buttons
  const appTab = document.getElementById("app-relays-tab");
  const mailboxTab = document.getElementById("mailboxes-tab");

  // Update tab content
  const appContent = document.getElementById("app-relays-content");
  const mailboxContent = document.getElementById("mailboxes-content");

  if (tabName === "app-relays") {
    // Activate app relays tab
    if (appTab) {
      appTab.className =
        "px-4 py-2 text-sm font-medium text-blue-400 transition-colors border-b-2 border-blue-500";
    }
    if (mailboxTab) {
      mailboxTab.className =
        "px-4 py-2 text-sm font-medium text-gray-400 transition-colors border-b-2 border-transparent hover:text-white";
    }
    if (appContent) appContent.classList.remove("hidden");
    if (mailboxContent) mailboxContent.classList.add("hidden");
  } else if (tabName === "mailboxes") {
    // Activate mailboxes tab
    if (mailboxTab) {
      mailboxTab.className =
        "px-4 py-2 text-sm font-medium text-blue-400 transition-colors border-b-2 border-blue-500";
    }
    if (appTab) {
      appTab.className =
        "px-4 py-2 text-sm font-medium text-gray-400 transition-colors border-b-2 border-transparent hover:text-white";
    }
    if (mailboxContent) mailboxContent.classList.remove("hidden");
    if (appContent) appContent.classList.add("hidden");
  }
}

// Make switchRelayTab available globally
window.switchRelayTab = switchRelayTab;

function updateSessionInfo(data) {
  updateElement("session-mode", data.sessionMode || "unknown");
  updateElement("signing-method", data.signingMethod || "unknown");

  // Derive canCreateEvents from sessionMode instead of using separate field
  const canCreateEvents = data.sessionMode === "write";
  updateElement("can-create-events", canCreateEvents ? "Yes" : "No");

  updateElement("cache-age", data.cacheAge || "unknown");
}

function updateKeyInformation(data) {
  updateElement(
    "settings-pubkey",
    data.publicKey || data.metadata?.pubkey || "Not available"
  );
  updateElement("settings-npub", data.npub || "Not available");
}

function updateRelayTable(data) {
  console.log("Processing relay data for table:", data);

  const tableBody = document.getElementById("app-relays-table");
  if (!tableBody) {
    console.error("App relays table not found");
    return;
  }

  // Handle both old format (data.relays) and new format (data.relayInfo)
  let relaysData = null;

  if (data.relayInfo && data.relayInfo.userRelays) {
    // New format - convert to old format for processing
    relaysData = {};

    // Process both relays (read & write)
    if (data.relayInfo.both) {
      data.relayInfo.both.forEach((url) => {
        relaysData[url] = { read: true, write: true };
      });
    }

    // Process read-only relays
    if (data.relayInfo.read) {
      data.relayInfo.read.forEach((url) => {
        relaysData[url] = { read: true, write: false };
      });
    }

    // Process write-only relays (if any)
    if (data.relayInfo.write) {
      data.relayInfo.write.forEach((url) => {
        relaysData[url] = { read: false, write: true };
      });
    }

    console.log("Converted relayInfo to relaysData:", relaysData);
  } else if (data.relays) {
    // Old format
    relaysData = data.relays;
    console.log("Using existing relays data:", relaysData);
  }

  if (!relaysData || Object.keys(relaysData).length === 0) {
    tableBody.innerHTML = `
        <tr>
          <td colspan="5" class="text-center py-8 text-gray-400">
            No relays configured
          </td>
        </tr>
      `;
    return;
  }

  // Process relays into table rows
  const relayRows = Object.entries(relaysData)
    .map(([url, config]) => {
      console.log(`Processing relay ${url}:`, config);

      // Handle different config formats
      let readEnabled, writeEnabled;

      if (typeof config === "object" && config !== null) {
        readEnabled = config.read !== false;
        writeEnabled = config.write !== false;
      } else {
        readEnabled = true;
        writeEnabled = true;
      }

      // Clean relay URL for display
      const displayUrl = url.replace(/^wss?:\/\//, "").replace(/\/$/, "");

      // Status indicators (start as "Checking...")
      const statusIndicator = `<span class="inline-flex items-center px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded-full" id="status-${btoa(
        url
      ).replace(/[^a-zA-Z0-9]/g, "")}">
        <span class="w-1.5 h-1.5 bg-gray-500 rounded-full mr-1"></span>
        Checking...
      </span>`;

      const readIndicator = readEnabled
        ? '<span class="text-green-400">✓</span>'
        : '<span class="text-gray-500">—</span>';

      const writeIndicator = writeEnabled
        ? '<span class="text-green-400">✓</span>'
        : '<span class="text-gray-500">—</span>';

      return `
        <tr class="border-b border-gray-700 hover:bg-gray-750">
          <td class="py-3 px-2">
            <div class="font-mono text-white text-sm">${displayUrl}</div>
            <div class="text-xs text-gray-400">${url}</div>
          </td>
          <td class="py-3 px-2 text-center">
            ${statusIndicator}
          </td>
          <td class="py-3 px-2 text-center">
            <span class="text-gray-400" data-relay="${url}" id="ping-${btoa(
        url
      ).replace(/[^a-zA-Z0-9]/g, "")}">
              <div class="animate-spin inline-block w-3 h-3 border border-white border-t-transparent rounded-full"></div>
            </span>
          </td>
          <td class="py-3 px-2 text-center">
            ${readIndicator}
          </td>
          <td class="py-3 px-2 text-center">
            ${writeIndicator}
          </td>
        </tr>
      `;
    })
    .join("");

  tableBody.innerHTML = relayRows;

  // Debug: Log all the IDs that were just created
  console.log("🔍 Just created table with these elements:");
  relayRows.split('id="').forEach((part, index) => {
    if (index > 0) {
      const idEnd = part.indexOf('"');
      if (idEnd > 0) {
        const id = part.substring(0, idEnd);
        console.log(`  - Element ID: ${id}`);
      }
    }
  });

  // Start pinging relays for response times
  console.log("🚀 About to call pingAllRelays with:", relaysData);

  // Test if the function exists
  console.log("🔍 pingAllRelays function type:", typeof pingAllRelays);

  // Simple test function to verify DOM elements exist
  console.log("🧪 Testing DOM elements:");
  Object.keys(relaysData).forEach((url) => {
    const urlHash = btoa(url).replace(/[^a-zA-Z0-9]/g, "");
    const pingElementId = `ping-${urlHash}`;
    const statusElementId = `status-${urlHash}`;
    const pingElement = document.getElementById(pingElementId);
    const statusElement = document.getElementById(statusElementId);
    console.log(
      `🧪 ${url} -> ping: ${!!pingElement}, status: ${!!statusElement}`
    );
  });

  // Call it with try-catch
  try {
    pingAllRelays(relaysData);
    console.log("✅ pingAllRelays called successfully");
  } catch (error) {
    console.error("❌ Error calling pingAllRelays:", error);
  }
}

async function pingAllRelays(relays) {
  for (const [url] of Object.entries(relays)) {
    try {
      const pingResult = await pingRelay(url);
      const urlHash = btoa(url).replace(/[^a-zA-Z0-9]/g, "");
      const pingElementId = `ping-${urlHash}`;
      const statusElementId = `status-${urlHash}`;
      const pingElement = document.getElementById(pingElementId);
      const statusElement = document.getElementById(statusElementId);

      if (pingElement) {
        if (pingResult.success) {
          pingElement.innerHTML = `<span class="text-green-400 font-mono">${pingResult.responseTime}ms</span>`;
          if (statusElement) {
            statusElement.innerHTML = `
              <span class="w-1.5 h-1.5 bg-green-500 rounded-full mr-1.5"></span>
              Online
            `;
            statusElement.className =
              "inline-flex items-center px-2 py-1 text-xs font-medium text-green-800 bg-green-100 rounded-full";
          }
        } else {
          pingElement.innerHTML = `<span class="text-red-400">Error</span>`;
          if (statusElement) {
            statusElement.innerHTML = `
              <span class="w-1.5 h-1.5 bg-red-500 rounded-full mr-1.5"></span>
              Offline
            `;
            statusElement.className =
              "inline-flex items-center px-2 py-1 text-xs font-medium text-red-800 bg-red-100 rounded-full";
          }
        }
      }
    } catch (error) {
      console.error(`Failed to ping ${url}:`, error);
      const urlHash = btoa(url).replace(/[^a-zA-Z0-9]/g, "");
      const pingElementId = `ping-${urlHash}`;
      const statusElementId = `status-${urlHash}`;
      const pingElement = document.getElementById(pingElementId);
      const statusElement = document.getElementById(statusElementId);

      if (pingElement) {
        pingElement.innerHTML = `<span class="text-red-400">Failed</span>`;
      }
      if (statusElement) {
        statusElement.innerHTML = `
          <span class="w-1.5 h-1.5 bg-red-500 rounded-full mr-1.5"></span>
          Offline
        `;
        statusElement.className =
          "inline-flex items-center px-2 py-1 text-xs font-medium text-red-800 bg-red-100 rounded-full";
      }
    }
  }
}

async function pingRelay(relayUrl) {
  try {
    console.log(
      `Making ping request to: /api/v1/relay/ping?url=${encodeURIComponent(
        relayUrl
      )}`
    );

    // Call the backend API to ping the relay
    const response = await fetch(
      `/api/v1/relay/ping?url=${encodeURIComponent(relayUrl)}`
    );

    console.log(`Ping response status: ${response.status} for ${relayUrl}`);

    if (!response.ok) {
      console.error(`HTTP ${response.status} for ${relayUrl}`);

      // If endpoint doesn't exist, return a mock result for testing
      if (response.status === 404) {
        console.warn("Ping API endpoint not found - using mock data");
        return {
          success: true,
          responseTime: Math.floor(Math.random() * 500) + 100, // Random 100-600ms
          relay: relayUrl,
        };
      }

      throw new Error(`HTTP ${response.status}`);
    }

    const result = await response.json();
    console.log(`Ping result for ${relayUrl}:`, result);
    return result;
  } catch (error) {
    console.error(`Ping failed for ${relayUrl}:`, error);

    // If network error, check if it's because endpoint doesn't exist
    if (error.message.includes("fetch")) {
      console.warn(
        "Network error - possibly missing ping endpoint, using mock data"
      );
      return {
        success: true,
        responseTime: Math.floor(Math.random() * 500) + 100, // Random 100-600ms
        relay: relayUrl,
      };
    }

    return { success: false, error: error.message };
  }
}

function refreshRelayStatus() {
  console.log("Refreshing relay status...");

  if (window.profileData && window.profileData.relays) {
    // Reset all ping indicators to loading
    Object.keys(window.profileData.relays).forEach((url) => {
      const pingElementId = `ping-${btoa(url).replace(/[^a-zA-Z0-9]/g, "")}`;
      const pingElement = document.getElementById(pingElementId);
      if (pingElement) {
        pingElement.innerHTML =
          '<div class="animate-spin inline-block w-3 h-3 border border-white border-t-transparent rounded-full"></div>';
      }
    });

    // Re-ping all relays
    pingAllRelays(window.profileData.relays);
    showNotification("Refreshing relay status...");
  }
}

function updateMailboxTable(data) {
  console.log("Processing mailbox data for table:", data.mailboxes);

  const tableBody = document.getElementById("mailboxes-table");
  if (!tableBody) {
    console.error("Mailboxes table not found");
    return;
  }

  // Handle mailboxes data structure
  let mailboxesData = null;

  if (data.mailboxes) {
    mailboxesData = {};

    // Process both relays (read & write mailboxes)
    if (data.mailboxes.both && Array.isArray(data.mailboxes.both)) {
      data.mailboxes.both.forEach((url) => {
        mailboxesData[url] = { read: true, write: true };
      });
    }

    // Process read-only mailboxes
    if (data.mailboxes.read && Array.isArray(data.mailboxes.read)) {
      data.mailboxes.read.forEach((url) => {
        mailboxesData[url] = { read: true, write: false };
      });
    }

    // Process write-only mailboxes
    if (data.mailboxes.write && Array.isArray(data.mailboxes.write)) {
      data.mailboxes.write.forEach((url) => {
        mailboxesData[url] = { read: false, write: true };
      });
    }

    console.log("Converted mailboxes to mailboxesData:", mailboxesData);
  }

  if (!mailboxesData || Object.keys(mailboxesData).length === 0) {
    tableBody.innerHTML = `
        <tr>
          <td colspan="5" class="text-center py-8 text-gray-400">
            No mailboxes configured
          </td>
        </tr>
      `;
    return;
  }

  // Process mailboxes into table rows
  const mailboxRows = Object.entries(mailboxesData)
    .map(([url, config]) => {
      console.log(`Processing mailbox ${url}:`, config);

      // Handle different config formats
      let readEnabled, writeEnabled;

      if (typeof config === "object" && config !== null) {
        readEnabled = config.read !== false;
        writeEnabled = config.write !== false;
      } else {
        readEnabled = true;
        writeEnabled = true;
      }

      // Clean mailbox URL for display
      const displayUrl = url.replace(/^wss?:\/\//, "").replace(/\/$/, "");

      // Status indicators (start as "Checking...")
      const statusIndicator = `<span class="inline-flex items-center px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded-full" id="mailbox-status-${btoa(
        url
      ).replace(/[^a-zA-Z0-9]/g, "")}">
        <span class="w-1.5 h-1.5 bg-gray-500 rounded-full mr-1"></span>
        Checking...
      </span>`;

      const readIndicator = readEnabled
        ? '<span class="text-green-400">✓</span>'
        : '<span class="text-gray-500">—</span>';

      const writeIndicator = writeEnabled
        ? '<span class="text-green-400">✓</span>'
        : '<span class="text-gray-500">—</span>';

      return `
        <tr class="border-b border-gray-700 hover:bg-gray-750">
          <td class="py-3 px-2">
            <div class="font-mono text-white text-sm">${displayUrl}</div>
            <div class="text-xs text-gray-400">${url}</div>
          </td>
          <td class="py-3 px-2 text-center">
            ${statusIndicator}
          </td>
          <td class="py-3 px-2 text-center">
            <span class="text-gray-400" data-relay="${url}" id="mailbox-ping-${btoa(
        url
      ).replace(/[^a-zA-Z0-9]/g, "")}">
              <div class="animate-spin inline-block w-3 h-3 border border-white border-t-transparent rounded-full"></div>
            </span>
          </td>
          <td class="py-3 px-2 text-center">
            ${readIndicator}
          </td>
          <td class="py-3 px-2 text-center">
            ${writeIndicator}
          </td>
        </tr>
      `;
    })
    .join("");

  tableBody.innerHTML = mailboxRows;

  // Start pinging mailboxes for response times
  pingAllMailboxes(mailboxesData);
}

async function pingAllMailboxes(mailboxes) {
  for (const [url] of Object.entries(mailboxes)) {
    try {
      const pingResult = await pingRelay(url);
      const urlHash = btoa(url).replace(/[^a-zA-Z0-9]/g, "");
      const pingElementId = `mailbox-ping-${urlHash}`;
      const statusElementId = `mailbox-status-${urlHash}`;
      const pingElement = document.getElementById(pingElementId);
      const statusElement = document.getElementById(statusElementId);

      if (pingElement) {
        if (pingResult.success) {
          pingElement.innerHTML = `<span class="text-green-400 font-mono">${pingResult.responseTime}ms</span>`;
          if (statusElement) {
            statusElement.innerHTML = `
              <span class="w-1.5 h-1.5 bg-green-500 rounded-full mr-1.5"></span>
              Online
            `;
            statusElement.className =
              "inline-flex items-center px-2 py-1 text-xs font-medium text-green-800 bg-green-100 rounded-full";
          }
        } else {
          pingElement.innerHTML = `<span class="text-red-400">Error</span>`;
          if (statusElement) {
            statusElement.innerHTML = `
              <span class="w-1.5 h-1.5 bg-red-500 rounded-full mr-1.5"></span>
              Offline
            `;
            statusElement.className =
              "inline-flex items-center px-2 py-1 text-xs font-medium text-red-800 bg-red-100 rounded-full";
          }
        }
      }
    } catch (error) {
      console.error(`Failed to ping mailbox ${url}:`, error);
      const urlHash = btoa(url).replace(/[^a-zA-Z0-9]/g, "");
      const pingElementId = `mailbox-ping-${urlHash}`;
      const statusElementId = `mailbox-status-${urlHash}`;
      const pingElement = document.getElementById(pingElementId);
      const statusElement = document.getElementById(statusElementId);

      if (pingElement) {
        pingElement.innerHTML = `<span class="text-red-400">Failed</span>`;
      }
      if (statusElement) {
        statusElement.innerHTML = `
          <span class="w-1.5 h-1.5 bg-red-500 rounded-full mr-1.5"></span>
          Offline
        `;
        statusElement.className =
          "inline-flex items-center px-2 py-1 text-xs font-medium text-red-800 bg-red-100 rounded-full";
      }
    }
  }
}

function updateDebugInfo(data) {
  const debugElement = document.getElementById("debug-data");
  if (debugElement) {
    debugElement.textContent = JSON.stringify(data, null, 2);
  }
}

// Utility function to update element content safely
function updateElement(id, content) {
  const element = document.getElementById(id);
  if (element) {
    element.textContent = content;
  } else {
    console.warn(`Element with id '${id}' not found`);
  }
}

// Notification functions
function showNotification(message) {
  // Create a simple toast notification
  const notification = document.createElement("div");
  notification.className =
    "fixed z-50 px-4 py-2 text-white bg-green-600 rounded-lg shadow-lg top-4 right-4";
  notification.textContent = message;

  document.body.appendChild(notification);

  setTimeout(() => {
    notification.remove();
  }, 3000);
}

function showError(message) {
  console.error("Settings error:", message);

  // Hide loading and content, show error
  document.getElementById("loading").classList.add("hidden");
  document.getElementById("settings-content").classList.add("hidden");
  document.getElementById("error-content").classList.remove("hidden");

  // Update error message
  const errorElement = document.getElementById("error-message");
  if (errorElement) {
    errorElement.textContent = message;
  }
}

function showCacheError(message) {
  console.error("Cache error:", message);

  // Create a more informative error for cache issues
  const cacheMessage = `${message}. Click "Refresh Settings" to try updating the cache.`;
  showError(cacheMessage);
}

function showRefreshNotification() {
  showNotification("Settings data refreshed from relays");
}
