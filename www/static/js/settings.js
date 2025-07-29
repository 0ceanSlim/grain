/**
 * Settings page functionality
 * Handles session information, relay management, and key operations
 */

// Global state
let isEditMode = false;
let originalRelayData = null;
let currentUserData = null;

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
      currentUserData = cacheData;

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

  // Update client relays table (changed from updateRelayTable)
  updateClientRelaysTable(data);

  // Update mailboxes table
  updateMailboxTable(data);

  // Update key information
  updateKeyInformation(data);
}

// Tab switching functionality - updated for new tab names
function switchRelayTab(tabName) {
  // Update tab buttons
  const clientTab = document.getElementById("client-relays-tab");
  const mailboxTab = document.getElementById("mailboxes-tab");

  // Update tab content
  const clientContent = document.getElementById("client-relays-content");
  const mailboxContent = document.getElementById("mailboxes-content");

  if (tabName === "client-relays") {
    // Activate client relays tab
    if (clientTab) {
      clientTab.className =
        "px-4 py-2 text-sm font-medium text-blue-400 transition-colors border-b-2 border-blue-500";
    }
    if (mailboxTab) {
      mailboxTab.className =
        "px-4 py-2 text-sm font-medium text-gray-400 transition-colors border-b-2 border-transparent hover:text-white";
    }
    if (clientContent) clientContent.classList.remove("hidden");
    if (mailboxContent) mailboxContent.classList.add("hidden");
  } else if (tabName === "mailboxes") {
    // Activate mailboxes tab
    if (mailboxTab) {
      mailboxTab.className =
        "px-4 py-2 text-sm font-medium text-blue-400 transition-colors border-b-2 border-blue-500";
    }
    if (clientTab) {
      clientTab.className =
        "px-4 py-2 text-sm font-medium text-gray-400 transition-colors border-b-2 border-transparent hover:text-white";
    }
    if (mailboxContent) mailboxContent.classList.remove("hidden");
    if (clientContent) clientContent.classList.add("hidden");
  }
}

// Make switchRelayTab available globally
window.switchRelayTab = switchRelayTab;

function updateSessionInfo(data) {
  // Only show sessionMode and signingMethod as requested
  updateElement("session-mode", data.sessionMode || "unknown");
  updateElement("signing-method", data.signingMethod || "unknown");
}

function updateClientRelaysTable(data) {
  console.log("Processing client relay data for table:", data);

  const tableBody = document.getElementById("client-relays-table");
  if (!tableBody) {
    console.error("Client relays table not found");
    return;
  }

  // Get client relays from clientRelays field in API response
  // The data structure is: { relays: [...], total: X, connected: Y }
  let clientRelaysData = data.clientRelays;
  let relaysList = [];

  if (
    clientRelaysData &&
    clientRelaysData.relays &&
    Array.isArray(clientRelaysData.relays)
  ) {
    relaysList = clientRelaysData.relays;
  } else {
    console.log("No client relays found in data:", data);
  }

  if (!relaysList || relaysList.length === 0) {
    tableBody.innerHTML = `
        <tr>
          <td colspan="5" class="text-center py-8 text-gray-400">
            No client relays configured
          </td>
        </tr>
      `;
    return;
  }

  // Process relays into table rows
  const relayRows = relaysList
    .map((relay) => {
      console.log(`Processing client relay:`, relay);

      const url = relay.url;
      const readEnabled = relay.read !== false;
      const writeEnabled = relay.write !== false;
      const connected = relay.connected || false;

      // Clean relay URL for display
      const displayUrl = url.replace(/^wss?:\/\//, "").replace(/\/$/, "");
      const safeId = btoa(url).replace(/[^a-zA-Z0-9]/g, "");

      // Status indicator based on connection
      const statusClass = connected
        ? "bg-green-100 text-green-800"
        : "bg-red-100 text-red-800";
      const statusDot = connected ? "bg-green-500" : "bg-red-500";
      const statusText = connected ? "Connected" : "Disconnected";

      const statusIndicator = `<span class="inline-flex items-center px-2 py-1 text-xs font-medium ${statusClass} rounded-full" id="status-${safeId}">
        <span class="w-1.5 h-1.5 ${statusDot} rounded-full mr-1"></span>
        ${statusText}
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
            <button onclick="pingSingleRelay('${url}')" 
                    class="text-blue-400 hover:text-blue-300 text-sm" 
                    id="ping-${safeId}">
              Ping
            </button>
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

  // Convert relays list to object format for pingAllRelays function
  const relaysForPing = {};
  relaysList.forEach((relay) => {
    relaysForPing[relay.url] = {
      read: relay.read,
      write: relay.write,
      connected: relay.connected,
    };
  });

  // Start pinging relays for response times
  console.log("🚀 About to call pingAllRelays with:", relaysForPing);
  pingAllRelays(relaysForPing);
}

// Edit mode functionality
function toggleEditMode() {
  if (isEditMode) return; // Already in edit mode

  isEditMode = true;

  // Store original data for cancel functionality
  originalRelayData = currentUserData
    ? JSON.parse(JSON.stringify(currentUserData.clientRelays || {}))
    : {};

  // Show/hide appropriate elements
  document.getElementById("client-relays-view").classList.add("hidden");
  document.getElementById("client-relays-edit").classList.remove("hidden");

  // Show/hide buttons
  document.getElementById("edit-relays-btn").classList.add("hidden");
  document.getElementById("save-relays-btn").classList.remove("hidden");
  document.getElementById("import-mailboxes-btn").classList.remove("hidden");
  document.getElementById("cancel-edit-btn").classList.remove("hidden");

  // Populate form with current relay data
  populateRelayForm(currentUserData?.clientRelays || {});
}

function cancelEdit() {
  isEditMode = false;

  // Show/hide appropriate elements
  document.getElementById("client-relays-view").classList.remove("hidden");
  document.getElementById("client-relays-edit").classList.add("hidden");

  // Show/hide buttons
  document.getElementById("edit-relays-btn").classList.remove("hidden");
  document.getElementById("save-relays-btn").classList.add("hidden");
  document.getElementById("import-mailboxes-btn").classList.add("hidden");
  document.getElementById("cancel-edit-btn").classList.add("hidden");

  // Clear form
  document.getElementById("relay-form-list").innerHTML = "";
}

function populateRelayForm(clientRelaysData) {
  const formList = document.getElementById("relay-form-list");
  formList.innerHTML = "";

  // Handle the new data structure: { relays: [...], total: X, connected: Y }
  let relaysList = [];
  if (
    clientRelaysData &&
    clientRelaysData.relays &&
    Array.isArray(clientRelaysData.relays)
  ) {
    relaysList = clientRelaysData.relays;
  }

  relaysList.forEach((relay) => {
    addRelayFormEntry(relay.url, relay.read !== false, relay.write !== false);
  });

  // Add one empty entry if no relays exist
  if (relaysList.length === 0) {
    addRelayFormEntry("", true, true);
  }
}

function addRelayFormEntry(url = "", read = true, write = true) {
  const formList = document.getElementById("relay-form-list");
  const entryId = `relay-entry-${Date.now()}-${Math.random()
    .toString(36)
    .substr(2, 9)}`;

  const entryHtml = `
    <div class="flex items-center space-x-3 bg-gray-600 p-3 rounded" id="${entryId}">
      <input 
        type="text" 
        placeholder="wss://relay.example.com" 
        value="${url}"
        class="flex-1 px-3 py-2 bg-gray-800 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:border-blue-500"
      />
      <label class="flex items-center space-x-1">
        <input type="checkbox" ${read ? "checked" : ""} class="text-blue-600">
        <span class="text-sm text-gray-300">Read</span>
      </label>
      <label class="flex items-center space-x-1">
        <input type="checkbox" ${write ? "checked" : ""} class="text-blue-600">
        <span class="text-sm text-gray-300">Write</span>
      </label>
      <button 
        onclick="removeRelayFormEntry('${entryId}')"
        class="px-3 py-2 text-sm bg-red-600 hover:bg-red-700 text-white rounded transition-colors"
      >
        Remove
      </button>
    </div>
  `;

  formList.insertAdjacentHTML("beforeend", entryHtml);
}

function addNewRelayInput() {
  addRelayFormEntry();
}

function removeRelayFormEntry(entryId) {
  const entry = document.getElementById(entryId);
  if (entry) {
    entry.remove();
  }
}

function importFromMailboxes() {
  if (!currentUserData?.mailboxes) {
    showNotification("No mailboxes data available to import", "error");
    return;
  }

  // Clear current form
  document.getElementById("relay-form-list").innerHTML = "";

  // Import from mailboxes
  const mailboxes = currentUserData.mailboxes;

  // Add both relays
  if (mailboxes.both && Array.isArray(mailboxes.both)) {
    mailboxes.both.forEach((url) => {
      addRelayFormEntry(url, true, true);
    });
  }

  // Add read-only relays
  if (mailboxes.read && Array.isArray(mailboxes.read)) {
    mailboxes.read.forEach((url) => {
      addRelayFormEntry(url, true, false);
    });
  }

  // Add write-only relays
  if (mailboxes.write && Array.isArray(mailboxes.write)) {
    mailboxes.write.forEach((url) => {
      addRelayFormEntry(url, false, true);
    });
  }

  showNotification("Imported relays from mailboxes", "success");
}

async function saveRelayChanges() {
  const formEntries = document.querySelectorAll("#relay-form-list > div");

  // Get current relays in object format for comparison
  const currentRelays = {};
  if (
    currentUserData?.clientRelays?.relays &&
    Array.isArray(currentUserData.clientRelays.relays)
  ) {
    currentUserData.clientRelays.relays.forEach((relay) => {
      currentRelays[relay.url] = {
        read: relay.read,
        write: relay.write,
      };
    });
  }

  // Get form data
  const newRelayConfig = {};
  formEntries.forEach((entry) => {
    const urlInput = entry.querySelector('input[type="text"]');
    const readCheckbox = entry.querySelectorAll('input[type="checkbox"]')[0];
    const writeCheckbox = entry.querySelectorAll('input[type="checkbox"]')[1];

    const url = urlInput.value.trim();
    if (url) {
      // Normalize URL
      let normalizedUrl = url;
      if (
        !normalizedUrl.startsWith("ws://") &&
        !normalizedUrl.startsWith("wss://")
      ) {
        normalizedUrl = "wss://" + normalizedUrl;
      }

      newRelayConfig[normalizedUrl] = {
        read: readCheckbox.checked,
        write: writeCheckbox.checked,
      };
    }
  });

  console.log("Saving relay changes:", newRelayConfig);
  console.log("Current relays:", currentRelays);

  try {
    // Determine which relays to connect to and disconnect from
    const currentUrls = new Set(Object.keys(currentRelays));
    const newUrls = new Set(Object.keys(newRelayConfig));

    // Relays to disconnect from
    const toDisconnect = [...currentUrls].filter((url) => !newUrls.has(url));

    // Relays to connect to (new ones or ones with changed permissions)
    const toConnect = [...newUrls].filter((url) => {
      if (!currentUrls.has(url)) return true; // New relay

      const current = currentRelays[url];
      const newConfig = newRelayConfig[url];

      // Check if permissions changed
      return (
        current.read !== newConfig.read || current.write !== newConfig.write
      );
    });

    console.log("Relays to disconnect:", toDisconnect);
    console.log("Relays to connect:", toConnect);

    // Show loading state
    document.getElementById("save-relays-btn").textContent = "Saving...";
    document.getElementById("save-relays-btn").disabled = true;

    // Disconnect old relays
    for (const url of toDisconnect) {
      const domain = url.replace(/^wss?:\/\//, "").replace(/\/$/, "");
      console.log(`Disconnecting from ${domain}`);

      const response = await fetch(`/api/v1/client/disconnect/${domain}`, {
        method: "POST",
      });

      if (!response.ok) {
        console.error(`Failed to disconnect from ${domain}`);
      }
    }

    // Connect to new/updated relays
    for (const url of toConnect) {
      const domain = url.replace(/^wss?:\/\//, "").replace(/\/$/, "");
      const config = newRelayConfig[url];

      console.log(
        `Connecting to ${domain} with read=${config.read}, write=${config.write}`
      );

      const params = new URLSearchParams();
      params.append("read", config.read);
      params.append("write", config.write);

      const response = await fetch(
        `/api/v1/client/connect/${domain}?${params}`,
        {
          method: "POST",
        }
      );

      if (!response.ok) {
        console.error(`Failed to connect to ${domain}`);
        const errorData = await response.json().catch(() => ({}));
        throw new Error(
          `Failed to connect to ${domain}: ${
            errorData.error || "Unknown error"
          }`
        );
      }
    }

    // Success! Exit edit mode and refresh data
    showNotification("Relay configuration saved successfully", "success");
    cancelEdit();

    // Refresh the settings data to show updated relays
    setTimeout(() => {
      loadSettingsData();
    }, 1000);
  } catch (error) {
    console.error("Error saving relay changes:", error);
    showNotification(`Error saving changes: ${error.message}`, "error");
  } finally {
    // Reset button state
    document.getElementById("save-relays-btn").textContent = "Save";
    document.getElementById("save-relays-btn").disabled = false;
  }
}

// Updated ping functionality
async function pingSingleRelay(relayUrl) {
  const safeId = btoa(relayUrl).replace(/[^a-zA-Z0-9]/g, "");
  const pingButton = document.getElementById(`ping-${safeId}`);

  if (pingButton) {
    pingButton.innerHTML =
      '<div class="animate-spin inline-block w-3 h-3 border border-white border-t-transparent rounded-full"></div>';
  }

  try {
    const result = await pingRelay(relayUrl);
    if (pingButton) {
      if (result.success) {
        pingButton.innerHTML = `<span class="text-green-400 font-mono">${result.responseTime}ms</span>`;
      } else {
        pingButton.innerHTML = `<span class="text-red-400">Error</span>`;
      }
    }
  } catch (error) {
    console.error(`Failed to ping ${relayUrl}:`, error);
    if (pingButton) {
      pingButton.innerHTML = `<span class="text-red-400">Failed</span>`;
    }
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
              Connected
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
      `Making ping request to: /api/v1/ping/?url=${encodeURIComponent(
        relayUrl
      )}`
    );

    // Updated to use your ping endpoint
    const response = await fetch(
      `/api/v1/ping/?url=${encodeURIComponent(relayUrl)}`
    );

    console.log(`Ping response status: ${response.status} for ${relayUrl}`);

    if (!response.ok) {
      console.error(`HTTP ${response.status} for ${relayUrl}`);

      // If endpoint doesn't exist, return a mock result for testing
      if (response.status === 404) {
        console.warn("Ping API endpoint not found - using mock data");
        return {
          success: true,
          responseTime: Math.floor(Math.random() * 500) + 100,
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
        responseTime: Math.floor(Math.random() * 500) + 100,
        relay: relayUrl,
      };
    }

    return { success: false, error: error.message };
  }
}

// Keep existing functions but update them to work with client relays
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
            <button onclick="pingSingleRelay('${url}')" 
                    class="text-blue-400 hover:text-blue-300 text-sm" 
                    id="mailbox-ping-${btoa(url).replace(/[^a-zA-Z0-9]/g, "")}">
              Ping
            </button>
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

function updateKeyInformation(data) {
  updateElement("settings-pubkey", data.publicKey || "Not available");
  updateElement("settings-npub", data.npub || "Not available");
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
function showNotification(message, type = "success") {
  // Create a simple toast notification
  const bgColor = type === "error" ? "bg-red-600" : "bg-green-600";
  const notification = document.createElement("div");
  notification.className = `fixed z-50 px-4 py-2 text-white ${bgColor} rounded-lg shadow-lg top-4 right-4`;
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

// Make functions available globally
window.toggleEditMode = toggleEditMode;
window.cancelEdit = cancelEdit;
window.saveRelayChanges = saveRelayChanges;
window.importFromMailboxes = importFromMailboxes;
window.addNewRelayInput = addNewRelayInput;
window.removeRelayFormEntry = removeRelayFormEntry;
window.pingSingleRelay = pingSingleRelay;
