/**
 * GRAIN Dashboard JavaScript - Phase 1 Redesign
 * Handles the new dashboard structure with focused configuration sections
 */

// New Dashboard Manager with reorganized structure
const newDashboardManager = {
  // API endpoints for the new dashboard structure
  endpoints: {
    // Core configs we're using
    server: "/api/v1/relay/config/server",
    rateLimit: "/api/v1/relay/config/rate_limit",
    eventTimeConstraints: "/api/v1/relay/config/event_time_constraints",
    eventPurge: "/api/v1/relay/config/event_purge",
    auth: "/api/v1/relay/config/auth",
    backupRelay: "/api/v1/relay/config/backup_relay",
    userSync: "/api/v1/relay/config/user_sync",
    whitelistKeys: "/api/v1/relay/keys/whitelist",
    blacklistKeys: "/api/v1/relay/keys/blacklist",
    whitelistConfig: "/api/v1/relay/config/whitelist",
    blacklistConfig: "/api/v1/relay/config/blacklist",
  },

  // Initialize dashboard
  init() {
    console.log("New Dashboard initializing...");
    this.updateTimestamp();
    this.setupEventListeners();
    this.refreshAll();
    this.startAutoRefresh();
  },

  // Update timestamp
  updateTimestamp() {
    const timestampEl = document.getElementById("last-updated");
    if (timestampEl) {
      timestampEl.textContent = new Date().toLocaleString();
    }
  },

  // Refresh all dashboard sections
  async refreshAll() {
    console.log("Refreshing new dashboard data...");

    const loadPromises = [
      this.loadRelayOverview(),
      this.loadPolicyLimits(),
      this.loadEventPurgeConfig(),
      this.loadSystemConfig(),
      this.loadUserSyncConfig(),
      this.loadWhitelistData(),
      this.loadBlacklistData(),
    ];

    try {
      await Promise.allSettled(loadPromises);
      this.updateTimestamp();
      console.log("Dashboard refresh completed");
    } catch (error) {
      console.error("Dashboard refresh error:", error);
    }
  },

  // Start auto-refresh every 30 seconds
  startAutoRefresh() {
    setInterval(() => {
      if (document.getElementById("relay-overview-content")) {
        console.log("Auto-refreshing dashboard...");
        this.refreshAll();
      }
    }, 30000);
  },

  // Generic fetch wrapper with error handling
  async fetchConfig(url, errorContainer) {
    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      return await response.json();
    } catch (error) {
      console.error(`Error fetching ${url}:`, error);
      if (errorContainer) {
        this.showError(
          errorContainer,
          `Failed to load configuration: ${error.message}`
        );
      }
      return null;
    }
  },

  // Show error message in container
  showError(containerId, message) {
    const container = document.getElementById(containerId);
    if (container) {
      container.innerHTML = `
        <div class="text-center text-red-400 py-4">
          <p>⚠️ ${message}</p>
          <button onclick="newDashboardManager.refreshAll()" class="mt-2 px-3 py-1 bg-red-600 hover:bg-red-700 rounded text-white text-sm">
            Retry
          </button>
        </div>
      `;
    }
  },

  // 1. Load Relay Overview (basic info - for Phase 2 we'll add NIP-11 data)
  async loadRelayOverview() {
    const container = document.getElementById("relay-overview-content");
    if (!container) return;

    // For now, show basic placeholder - Phase 2 will fetch NIP-11 data
    container.innerHTML = `
      <div class="space-y-4">
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Relay Status</span>
          <span class="inline-flex px-2 py-1 text-xs font-medium bg-green-100 text-green-800 rounded-full">
            Online
          </span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Software</span>
          <span class="text-white font-medium">🌾 GRAIN</span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Protocol</span>
          <span class="text-white font-medium">Nostr Relay</span>
        </div>
        <div class="text-sm text-gray-400 mt-4">
          <p>Detailed relay information will be available in the next update.</p>
        </div>
      </div>
    `;
  },

  // 2. Load Policy & Limits (rate limits + timeouts + time constraints)
  async loadPolicyLimits() {
    const [rateLimitData, serverData, timeConstraintsData] = await Promise.all([
      this.fetchConfig(this.endpoints.rateLimit, "policy-limits-table"),
      this.fetchConfig(this.endpoints.server, "policy-limits-table"),
      this.fetchConfig(
        this.endpoints.eventTimeConstraints,
        "policy-limits-table"
      ),
    ]);

    if (!rateLimitData || !serverData || !timeConstraintsData) return;

    const tbody = document.getElementById("policy-limits-table");
    if (!tbody) return;

    const rows = [
      // Rate limits
      {
        type: "WebSocket Messages",
        limit: `${rateLimitData.ws_limit}/sec`,
        burst: `${rateLimitData.ws_burst} messages`,
        status: this.getStatusBadge(true),
      },
      {
        type: "Event Publishing",
        limit: `${rateLimitData.event_limit}/sec`,
        burst: `${rateLimitData.event_burst} events`,
        status: this.getStatusBadge(true),
      },
      {
        type: "Query Requests",
        limit: `${rateLimitData.req_limit}/sec`,
        burst: `${rateLimitData.req_burst} queries`,
        status: this.getStatusBadge(true),
      },
      {
        type: "Max Event Size",
        limit: this.formatBytes(rateLimitData.max_event_size),
        burst: "Per event",
        status: this.getStatusBadge(true),
      },
      // Server timeouts (excluding port)
      {
        type: "Read Timeout",
        limit: `${serverData.read_timeout}s`,
        burst: "HTTP requests",
        status: this.getStatusBadge(true),
      },
      {
        type: "Write Timeout",
        limit: `${serverData.write_timeout}s`,
        burst: "HTTP responses",
        status: this.getStatusBadge(true),
      },
      {
        type: "Idle Timeout",
        limit: `${serverData.idle_timeout}s`,
        burst: "Connections",
        status: this.getStatusBadge(true),
      },
      {
        type: "Max Subscriptions",
        limit: `${serverData.max_subscriptions_per_client}`,
        burst: "Per client",
        status: this.getStatusBadge(true),
      },
      // Time constraints
      {
        type: "Future Events",
        limit: `${timeConstraintsData.max_created_at_future_seconds}s`,
        burst: "Max future time",
        status: this.getStatusBadge(true),
      },
      {
        type: "Past Events",
        limit: `${Math.floor(
          timeConstraintsData.max_created_at_past_seconds / 86400
        )} days`,
        burst: "Max past time",
        status: this.getStatusBadge(true),
      },
    ];

    tbody.innerHTML = rows
      .map(
        (row) => `
      <tr>
        <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-white">${row.type}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${row.limit}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${row.burst}</td>
        <td class="px-6 py-4 whitespace-nowrap">${row.status}</td>
      </tr>
    `
      )
      .join("");
  },

  // 3. Load Event Purge Management
  async loadEventPurgeConfig() {
    const data = await this.fetchConfig(
      this.endpoints.eventPurge,
      "event-purge-content"
    );
    if (!data) return;

    const container = document.getElementById("event-purge-content");
    if (!container) return;

    container.innerHTML = `
      <div class="space-y-4">
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Purge Enabled</span>
          <span class="inline-flex px-2 py-1 text-xs font-medium ${
            data.enabled
              ? "bg-green-100 text-green-800"
              : "bg-red-100 text-red-800"
          } rounded-full">
            ${data.enabled ? "Yes" : "No"}
          </span>
        </div>
        ${
          data.enabled
            ? `
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Keep Interval</span>
            <span class="text-white font-medium">${
              data.keep_interval_hours
            } hours</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Purge Interval</span>
            <span class="text-white font-medium">${
              data.purge_interval_minutes
            } minutes</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Exclude Whitelisted</span>
            <span class="text-white font-medium">${
              data.exclude_whitelisted ? "Yes" : "No"
            }</span>
          </div>
          ${
            data.kinds_to_purge && data.kinds_to_purge.length > 0
              ? `
            <div class="flex justify-between items-center">
              <span class="text-gray-300">Purge Kinds</span>
              <span class="text-white font-medium">${data.kinds_to_purge.join(
                ", "
              )}</span>
            </div>
          `
              : ""
          }
        `
            : ""
        }
      </div>
    `;
  },

  // 4. Load System Configuration (auth + backup relay)
  async loadSystemConfig() {
    const [authData, backupData] = await Promise.all([
      this.fetchConfig(this.endpoints.auth, "system-config-table"),
      this.fetchConfig(this.endpoints.backupRelay, "system-config-table"),
    ]);

    if (!authData || !backupData) return;

    const tbody = document.getElementById("system-config-table");
    if (!tbody) return;

    const rows = [
      {
        setting: "Authentication",
        value: authData.enabled ? "NIP-42 Enabled" : "Disabled",
        description: "Cryptographic user verification",
        status: this.getBooleanBadge(authData.enabled),
      },
      {
        setting: "Backup Relay",
        value: backupData.enabled ? "Enabled" : "Disabled",
        description: backupData.enabled
          ? `Connected to ${backupData.url}`
          : "No backup configured",
        status: this.getBooleanBadge(backupData.enabled),
      },
    ];

    tbody.innerHTML = rows
      .map(
        (row) => `
      <tr>
        <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-white">${row.setting}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${row.value}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-400">${row.description}</td>
        <td class="px-6 py-4 whitespace-nowrap">${row.status}</td>
      </tr>
    `
      )
      .join("");
  },

  // 5. Load User Sync Configuration (experimental section)
  async loadUserSyncConfig() {
    const data = await this.fetchConfig(
      this.endpoints.userSync,
      "user-sync-content"
    );
    if (!data) return;

    const container = document.getElementById("user-sync-content");
    if (!container) return;

    container.innerHTML = `
      <div class="space-y-4">
        <div class="bg-orange-900/20 border border-orange-700 rounded p-4">
          <div class="flex items-center space-x-2 mb-2">
            <span class="text-orange-400">⚠️</span>
            <span class="text-orange-300 font-medium">Experimental Feature</span>
          </div>
          <p class="text-sm text-orange-200">User sync is in development and may not function as expected.</p>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Sync Enabled</span>
          <span class="inline-flex px-2 py-1 text-xs font-medium ${
            data.enabled
              ? "bg-green-100 text-green-800"
              : "bg-gray-100 text-gray-800"
          } rounded-full">
            ${data.enabled ? "Yes" : "No"}
          </span>
        </div>
        ${
          data.enabled
            ? `
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Sync Interval</span>
            <span class="text-white font-medium">${data.interval_hours} hours</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Batch Size</span>
            <span class="text-white font-medium">${data.batch_size} users</span>
          </div>
        `
            : ""
        }
      </div>
    `;
  },

  // 6. Load Whitelist Data (simplified for Phase 1, enhanced in Phase 2)
  async loadWhitelistData() {
    const [keysData, configData] = await Promise.all([
      this.fetchConfig(this.endpoints.whitelistKeys, "whitelist-content"),
      this.fetchConfig(this.endpoints.whitelistConfig, "whitelist-content"),
    ]);

    if (!keysData || !configData) return;

    const container = document.getElementById("whitelist-content");
    if (!container) return;

    const totalKeys =
      (keysData.list?.length || 0) +
      (keysData.domains?.reduce(
        (acc, domain) => acc + (domain.pubkeys?.length || 0),
        0
      ) || 0);

    container.innerHTML = `
      <div class="space-y-4">
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Whitelist Status</span>
          <span class="inline-flex px-2 py-1 text-xs font-medium ${
            configData.pubkey_whitelist?.enabled
              ? "bg-green-100 text-green-800"
              : "bg-gray-100 text-gray-800"
          } rounded-full">
            ${configData.pubkey_whitelist?.enabled ? "Active" : "Inactive"}
          </span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Total Keys</span>
          <span class="text-white font-medium">${totalKeys}</span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Direct Keys</span>
          <span class="text-white font-medium">${
            keysData.list?.length || 0
          }</span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Domain Keys</span>
          <span class="text-white font-medium">${
            keysData.domains?.length || 0
          } domains</span>
        </div>
        <div class="text-sm text-gray-400 mt-4">
          <p>Key profiles and management will be available in the next update.</p>
        </div>
      </div>
    `;
  },

  // 7. Load Blacklist Data (simplified for Phase 1)
  async loadBlacklistData() {
    const [keysData, configData] = await Promise.all([
      this.fetchConfig(this.endpoints.blacklistKeys, "blacklist-content"),
      this.fetchConfig(this.endpoints.blacklistConfig, "blacklist-content"),
    ]);

    if (!keysData || !configData) return;

    const container = document.getElementById("blacklist-content");
    if (!container) return;

    const permanentCount = keysData.permanent?.length || 0;
    const temporaryCount = keysData.temporary?.length || 0;
    const mutelistCount = Object.keys(keysData.mutelist || {}).length;

    container.innerHTML = `
      <div class="space-y-4">
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Blacklist Status</span>
          <span class="inline-flex px-2 py-1 text-xs font-medium ${
            configData.enabled
              ? "bg-red-100 text-red-800"
              : "bg-gray-100 text-gray-800"
          } rounded-full">
            ${configData.enabled ? "Active" : "Inactive"}
          </span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Permanent Bans</span>
          <span class="text-white font-medium">${permanentCount}</span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Temporary Bans</span>
          <span class="text-white font-medium">${temporaryCount}</span>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-gray-300">Mute Lists</span>
          <span class="text-white font-medium">${mutelistCount}</span>
        </div>
      </div>
    `;
  },

  // Utility functions
  formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return "0 Bytes";
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ["Bytes", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
  },

  getStatusBadge(status) {
    const statusClass = status
      ? "bg-green-100 text-green-800"
      : "bg-red-100 text-red-800";
    const statusText = status ? "Active" : "Inactive";
    return `<span class="inline-flex px-2 py-1 text-xs font-medium ${statusClass} rounded-full">${statusText}</span>`;
  },

  getBooleanBadge(value) {
    const badgeClass = value
      ? "bg-green-100 text-green-800"
      : "bg-gray-100 text-gray-800";
    const text = value ? "Enabled" : "Disabled";
    return `<span class="inline-flex px-2 py-1 text-xs font-medium ${badgeClass} rounded-full">${text}</span>`;
  },
};

// Initialize dashboard when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => {
    newDashboardManager.init();
  });
} else {
  newDashboardManager.init();
}

// Expose globally for Hyperscript and compatibility
window.newDashboardManager = newDashboardManager;

// Legacy compatibility - keep old dashboard manager for transition
window.dashboardManager = newDashboardManager;

console.log("New Dashboard.js loaded successfully");
