/**
 * GRAIN Dashboard JavaScript
 * Handles all dashboard functionality with clean JavaScript
 */

// Dashboard Manager - handles all dashboard operations
const dashboardManager = {
  // API endpoints
  endpoints: {
    rateLimit: "/api/v1/relay/config/rate_limit",
    server: "/api/v1/relay/config/server",
    auth: "/api/v1/relay/config/auth",
    whitelist: "/api/v1/relay/whitelist",
    blacklist: "/api/v1/relay/blacklist",
    logging: "/api/v1/relay/config/logging",
    mongodb: "/api/v1/relay/config/mongodb",
    resourceLimits: "/api/v1/relay/config/resource_limits",
    eventPurge: "/api/v1/relay/config/event_purge",
  },

  // Initialize dashboard
  init() {
    console.log("Dashboard initializing...");
    this.updateTimestamp();
    this.setupEventListeners();
    this.refreshAll();
    this.startAutoRefresh();
  },

  // Setup event listeners
  setupEventListeners() {
    const refreshBtn = document.getElementById("refresh-dashboard-btn");
    if (refreshBtn) {
      refreshBtn.addEventListener("click", () => {
        this.refreshAll();
      });
    }
  },

  // Update timestamp
  updateTimestamp() {
    const timestampEl = document.getElementById("last-updated");
    if (timestampEl) {
      timestampEl.textContent = new Date().toLocaleString();
    }
  },

  // Refresh all configurations
  async refreshAll() {
    console.log("Refreshing all dashboard data...");

    const loadPromises = [
      this.loadRateLimitConfig(),
      this.loadServerConfig(),
      this.loadAuthConfig(),
      this.loadWhitelistData(),
      this.loadBlacklistData(),
      this.loadLoggingConfig(),
      this.loadMongoDBConfig(),
      this.loadResourceLimitsConfig(),
      this.loadEventPurgeConfig(),
    ];

    try {
      await Promise.allSettled(loadPromises);
      this.updateTimestamp();
      console.log("Dashboard refresh completed");
    } catch (error) {
      console.error("Dashboard refresh error:", error);
    }
  },

  // Generic fetch wrapper with error handling
  async fetchConfig(url, errorElement) {
    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      return await response.json();
    } catch (error) {
      console.error(`Error loading ${url}:`, error);
      const element = document.getElementById(errorElement);
      if (element) {
        element.innerHTML =
          '<p class="text-red-400">Error loading configuration</p>';
      }
      return null;
    }
  },

  // Load Rate Limit Configuration
  async loadRateLimitConfig() {
    const data = await this.fetchConfig(
      this.endpoints.rateLimit,
      "rate-limit-table"
    );
    if (data) {
      this.populateRateLimitTable(data);
    }
  },

  // Load Server Configuration
  async loadServerConfig() {
    const data = await this.fetchConfig(
      this.endpoints.server,
      "server-config-table"
    );
    if (data) {
      this.populateServerConfigTable(data);
    }
  },

  // Load Auth Configuration
  async loadAuthConfig() {
    const data = await this.fetchConfig(
      this.endpoints.auth,
      "auth-config-table"
    );
    if (data) {
      this.populateAuthConfigTable(data);
    }
  },

  // Load Whitelist Data
  async loadWhitelistData() {
    const data = await this.fetchConfig(
      this.endpoints.whitelist,
      "whitelist-content"
    );
    if (data) {
      this.populateWhitelistContent(data);
    }
  },

  // Load Blacklist Data
  async loadBlacklistData() {
    const data = await this.fetchConfig(
      this.endpoints.blacklist,
      "blacklist-content"
    );
    if (data) {
      this.populateBlacklistContent(data);
    }
  },

  // Load Logging Configuration
  async loadLoggingConfig() {
    const data = await this.fetchConfig(
      this.endpoints.logging,
      "logging-content"
    );
    if (data) {
      this.populateLoggingContent(data);
    }
  },

  // Load MongoDB Configuration
  async loadMongoDBConfig() {
    const data = await this.fetchConfig(
      this.endpoints.mongodb,
      "mongodb-content"
    );
    if (data) {
      this.populateMongoDBContent(data);
    }
  },

  // Load Resource Limits Configuration
  async loadResourceLimitsConfig() {
    const data = await this.fetchConfig(
      this.endpoints.resourceLimits,
      "resource-limits-content"
    );
    if (data) {
      this.populateResourceLimitsContent(data);
    }
  },

  // Load Event Purge Configuration
  async loadEventPurgeConfig() {
    const data = await this.fetchConfig(
      this.endpoints.eventPurge,
      "event-purge-content"
    );
    if (data) {
      this.populateEventPurgeContent(data);
    }
  },

  // Start auto-refresh timer
  startAutoRefresh() {
    setInterval(() => {
      console.log("Auto-refreshing dashboard...");
      this.refreshAll();
    }, 30000); // 30 seconds
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

  // Populate Rate Limit Table
  populateRateLimitTable(data) {
    const tbody = document.getElementById("rate-limit-table");
    if (!tbody || !data) return;

    const rows = [
      {
        type: "WebSocket Messages",
        limit: `${data.ws_limit}/sec`,
        burst: `${data.ws_burst} messages`,
        status: this.getStatusBadge(true),
      },
      {
        type: "Event Publishing",
        limit: `${data.event_limit}/sec`,
        burst: `${data.event_burst} events`,
        status: this.getStatusBadge(true),
      },
      {
        type: "Query Requests",
        limit: `${data.req_limit}/sec`,
        burst: `${data.req_burst} queries`,
        status: this.getStatusBadge(true),
      },
      {
        type: "Max Event Size",
        limit: this.formatBytes(data.max_event_size),
        burst: "Per event",
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

  // Populate Server Configuration Table
  populateServerConfigTable(data) {
    const tbody = document.getElementById("server-config-table");
    if (!tbody || !data) return;

    const rows = [
      {
        setting: "Server Port",
        value: data.port || "Not set",
        description: "HTTP server listening port",
        status: this.getStatusBadge(!!data.port),
      },
      {
        setting: "Read Timeout",
        value: `${data.read_timeout || 0}s`,
        description: "HTTP request read timeout",
        status: this.getStatusBadge(true),
      },
      {
        setting: "Write Timeout",
        value: `${data.write_timeout || 0}s`,
        description: "HTTP response write timeout",
        status: this.getStatusBadge(true),
      },
      {
        setting: "Idle Timeout",
        value: `${data.idle_timeout || 0}s`,
        description: "HTTP connection idle timeout",
        status: this.getStatusBadge(true),
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

  // Populate Auth Configuration Table
  populateAuthConfigTable(data) {
    const tbody = document.getElementById("auth-config-table");
    if (!tbody || !data) return;

    const rows = [
      {
        setting: "Authentication",
        value: data.enabled ? "NIP-42" : "Disabled",
        description: "Cryptographic user verification",
        status: this.getBooleanBadge(data.enabled),
      },
      {
        setting: "Required",
        value: data.required ? "Yes" : "No",
        description: "Authentication requirement for access",
        status: this.getBooleanBadge(data.required),
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

  // Populate Whitelist Content
  populateWhitelistContent(data) {
    const container = document.getElementById("whitelist-content");
    if (!container || !data) return;

    const pubkeyCount = data.pubkeys ? data.pubkeys.length : 0;

    container.innerHTML = `
            <div class="space-y-4">
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Whitelisted Users</span>
                    <span class="text-white font-medium">${pubkeyCount}</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Status</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full">
                        ${pubkeyCount > 0 ? "Active" : "Empty"}
                    </span>
                </div>
                ${
                  pubkeyCount > 0
                    ? `
                    <div class="text-xs text-gray-400">
                        <p>Recent entries:</p>
                        <div class="mt-2 space-y-1">
                            ${data.pubkeys
                              .slice(0, 3)
                              .map(
                                (pubkey) =>
                                  `<div class="font-mono text-gray-500">${pubkey.substring(
                                    0,
                                    16
                                  )}...</div>`
                              )
                              .join("")}
                        </div>
                    </div>
                `
                    : ""
                }
            </div>
        `;
  },

  // Populate Blacklist Content
  populateBlacklistContent(data) {
    const container = document.getElementById("blacklist-content");
    if (!container || !data) return;

    const permanentCount = data.permanent ? data.permanent.length : 0;
    const temporaryCount = data.temporary ? data.temporary.length : 0;
    const mutelistCount = data.mutelist ? Object.keys(data.mutelist).length : 0;

    container.innerHTML = `
            <div class="space-y-4">
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
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Status</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium bg-yellow-100 text-yellow-800 rounded-full">
                        ${
                          permanentCount + temporaryCount + mutelistCount > 0
                            ? "Active"
                            : "Empty"
                        }
                    </span>
                </div>
            </div>
        `;
  },

  // Populate Logging Content
  populateLoggingContent(data) {
    const container = document.getElementById("logging-content");
    if (!container || !data) return;

    container.innerHTML = `
            <div class="space-y-4">
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Log Level</span>
                    <span class="text-white font-medium">${
                      data.level || "Not set"
                    }</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Log File</span>
                    <span class="text-white font-medium">${
                      data.file || "Not set"
                    }</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Max Size</span>
                    <span class="text-white font-medium">${
                      data.max_log_size_mb || 0
                    } MB</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Structured</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium ${
                      data.structure
                        ? "bg-green-100 text-green-800"
                        : "bg-gray-100 text-gray-800"
                    } rounded-full">
                        ${data.structure ? "Enabled" : "Disabled"}
                    </span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Backup Count</span>
                    <span class="text-white font-medium">${
                      data.backup_count || 0
                    }</span>
                </div>
            </div>
        `;
  },

  // Populate MongoDB Content
  populateMongoDBContent(data) {
    const container = document.getElementById("mongodb-content");
    if (!container || !data) return;

    container.innerHTML = `
            <div class="space-y-4">
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Database</span>
                    <span class="text-white font-medium">${
                      data.database || "Not set"
                    }</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Connection</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium bg-green-100 text-green-800 rounded-full">
                        Connected
                    </span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Collections</span>
                    <span class="text-white font-medium">Per-kind optimization</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Indexing</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full">
                        Automatic
                    </span>
                </div>
            </div>
        `;
  },

  // Populate Resource Limits Content
  populateResourceLimitsContent(data) {
    const container = document.getElementById("resource-limits-content");
    if (!container || !data) return;

    container.innerHTML = `
            <div class="space-y-4">
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Max Connections</span>
                    <span class="text-white font-medium">${
                      data.max_connections || "Unlimited"
                    }</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Memory Limit</span>
                    <span class="text-white font-medium">${
                      data.memory_limit
                        ? this.formatBytes(data.memory_limit)
                        : "System default"
                    }</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">CPU Cores</span>
                    <span class="text-white font-medium">${
                      data.cpu_cores || "All available"
                    }</span>
                </div>
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Status</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium bg-green-100 text-green-800 rounded-full">
                        Optimized
                    </span>
                </div>
            </div>
        `;
  },

  // Populate Event Purge Content
  populateEventPurgeContent(data) {
    const container = document.getElementById("event-purge-content");
    if (!container || !data) return;

    const purgeEnabled = data.enabled !== false; // Default to enabled if not specified

    container.innerHTML = `
            <div class="space-y-4">
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Purge Enabled</span>
                    <span class="inline-flex px-2 py-1 text-xs font-medium ${
                      purgeEnabled
                        ? "bg-green-100 text-green-800"
                        : "bg-red-100 text-red-800"
                    } rounded-full">
                        ${purgeEnabled ? "Yes" : "No"}
                    </span>
                </div>
                ${
                  data.regular_retention_days
                    ? `
                    <div class="flex justify-between items-center">
                        <span class="text-gray-300">Regular Events</span>
                        <span class="text-white font-medium">${data.regular_retention_days} days</span>
                    </div>
                `
                    : ""
                }
                ${
                  data.replaceable_retention_days
                    ? `
                    <div class="flex justify-between items-center">
                        <span class="text-gray-300">Replaceable Events</span>
                        <span class="text-white font-medium">${data.replaceable_retention_days} days</span>
                    </div>
                `
                    : ""
                }
                ${
                  data.ephemeral_retention_hours
                    ? `
                    <div class="flex justify-between items-center">
                        <span class="text-gray-300">Ephemeral Events</span>
                        <span class="text-white font-medium">${data.ephemeral_retention_hours} hours</span>
                    </div>
                `
                    : ""
                }
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Schedule</span>
                    <span class="text-white font-medium">${
                      data.schedule || "Daily"
                    }</span>
                </div>
            </div>
        `;
  },
};

// Initialize dashboard when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => {
    dashboardManager.init();
  });
} else {
  dashboardManager.init();
}

// Expose for global access and Hyperscript
window.dashboardManager = dashboardManager;

// Legacy function names for compatibility
window.populateRateLimitTable = (data) =>
  dashboardManager.populateRateLimitTable(data);
window.populateServerConfigTable = (data) =>
  dashboardManager.populateServerConfigTable(data);
window.populateAuthConfigTable = (data) =>
  dashboardManager.populateAuthConfigTable(data);
window.populateWhitelistContent = (data) =>
  dashboardManager.populateWhitelistContent(data);
window.populateBlacklistContent = (data) =>
  dashboardManager.populateBlacklistContent(data);
window.populateLoggingContent = (data) =>
  dashboardManager.populateLoggingContent(data);
window.populateMongoDBContent = (data) =>
  dashboardManager.populateMongoDBContent(data);
window.populateResourceLimitsContent = (data) =>
  dashboardManager.populateResourceLimitsContent(data);
window.populateEventPurgeContent = (data) =>
  dashboardManager.populateEventPurgeContent(data);

console.log("Dashboard.js loaded successfully");
/**
 * GRAIN Dashboard JavaScript
 * Handles populating dashboard tables and content from API responses
 */

// Utility functions
window.dashboardUtils = {
  formatBytes: function (bytes, decimals = 2) {
    if (bytes === 0) return "0 Bytes";
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ["Bytes", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
  },

  getStatusBadge: function (status) {
    const statusClass = status
      ? "bg-green-100 text-green-800"
      : "bg-red-100 text-red-800";
    const statusText = status ? "Active" : "Inactive";
    return `<span class="inline-flex px-2 py-1 text-xs font-medium ${statusClass} rounded-full">${statusText}</span>`;
  },

  getBooleanBadge: function (value) {
    const badgeClass = value
      ? "bg-green-100 text-green-800"
      : "bg-gray-100 text-gray-800";
    const text = value ? "Enabled" : "Disabled";
    return `<span class="inline-flex px-2 py-1 text-xs font-medium ${badgeClass} rounded-full">${text}</span>`;
  },
};

// Rate Limit Configuration
window.populateRateLimitTable = function (data) {
  const tbody = document.getElementById("rate-limit-table");
  if (!tbody || !data) return;

  const rows = [
    {
      type: "WebSocket Messages",
      limit: `${data.ws_limit}/sec`,
      burst: `${data.ws_burst} messages`,
      status: window.dashboardUtils.getStatusBadge(true),
    },
    {
      type: "Event Publishing",
      limit: `${data.event_limit}/sec`,
      burst: `${data.event_burst} events`,
      status: window.dashboardUtils.getStatusBadge(true),
    },
    {
      type: "Query Requests",
      limit: `${data.req_limit}/sec`,
      burst: `${data.req_burst} queries`,
      status: window.dashboardUtils.getStatusBadge(true),
    },
    {
      type: "Max Event Size",
      limit: window.dashboardUtils.formatBytes(data.max_event_size),
      burst: "Per event",
      status: window.dashboardUtils.getStatusBadge(true),
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
};

// Server Configuration
window.populateServerConfigTable = function (data) {
  const tbody = document.getElementById("server-config-table");
  if (!tbody || !data) return;

  const rows = [
    {
      setting: "Server Port",
      value: data.port || "Not set",
      description: "HTTP server listening port",
      status: window.dashboardUtils.getStatusBadge(!!data.port),
    },
    {
      setting: "Read Timeout",
      value: `${data.read_timeout || 0}s`,
      description: "HTTP request read timeout",
      status: window.dashboardUtils.getStatusBadge(true),
    },
    {
      setting: "Write Timeout",
      value: `${data.write_timeout || 0}s`,
      description: "HTTP response write timeout",
      status: window.dashboardUtils.getStatusBadge(true),
    },
    {
      setting: "Idle Timeout",
      value: `${data.idle_timeout || 0}s`,
      description: "HTTP connection idle timeout",
      status: window.dashboardUtils.getStatusBadge(true),
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
};

// Authentication Configuration
window.populateAuthConfigTable = function (data) {
  const tbody = document.getElementById("auth-config-table");
  if (!tbody || !data) return;

  const rows = [
    {
      setting: "Authentication",
      value: data.enabled ? "NIP-42" : "Disabled",
      description: "Cryptographic user verification",
      status: window.dashboardUtils.getBooleanBadge(data.enabled),
    },
    {
      setting: "Required",
      value: data.required ? "Yes" : "No",
      description: "Authentication requirement for access",
      status: window.dashboardUtils.getBooleanBadge(data.required),
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
};

// Whitelist Content
window.populateWhitelistContent = function (data) {
  const container = document.getElementById("whitelist-content");
  if (!container || !data) return;

  const pubkeyCount = data.pubkeys ? data.pubkeys.length : 0;

  container.innerHTML = `
        <div class="space-y-4">
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Whitelisted Users</span>
                <span class="text-white font-medium">${pubkeyCount}</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Status</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full">
                    ${pubkeyCount > 0 ? "Active" : "Empty"}
                </span>
            </div>
            ${
              pubkeyCount > 0
                ? `
                <div class="text-xs text-gray-400">
                    <p>Recent entries:</p>
                    <div class="mt-2 space-y-1">
                        ${data.pubkeys
                          .slice(0, 3)
                          .map(
                            (pubkey) =>
                              `<div class="font-mono text-gray-500">${pubkey.substring(
                                0,
                                16
                              )}...</div>`
                          )
                          .join("")}
                    </div>
                </div>
            `
                : ""
            }
        </div>
    `;
};

// Blacklist Content
window.populateBlacklistContent = function (data) {
  const container = document.getElementById("blacklist-content");
  if (!container || !data) return;

  const permanentCount = data.permanent ? data.permanent.length : 0;
  const temporaryCount = data.temporary ? data.temporary.length : 0;
  const mutelistCount = data.mutelist ? Object.keys(data.mutelist).length : 0;

  container.innerHTML = `
        <div class="space-y-4">
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
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Status</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium bg-yellow-100 text-yellow-800 rounded-full">
                    ${
                      permanentCount + temporaryCount + mutelistCount > 0
                        ? "Active"
                        : "Empty"
                    }
                </span>
            </div>
        </div>
    `;
};

// Logging Configuration
window.populateLoggingContent = function (data) {
  const container = document.getElementById("logging-content");
  if (!container || !data) return;

  container.innerHTML = `
        <div class="space-y-4">
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Log Level</span>
                <span class="text-white font-medium">${
                  data.level || "Not set"
                }</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Log File</span>
                <span class="text-white font-medium">${
                  data.file || "Not set"
                }</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Max Size</span>
                <span class="text-white font-medium">${
                  data.max_log_size_mb || 0
                } MB</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Structured</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium ${
                  data.structure
                    ? "bg-green-100 text-green-800"
                    : "bg-gray-100 text-gray-800"
                } rounded-full">
                    ${data.structure ? "Enabled" : "Disabled"}
                </span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Backup Count</span>
                <span class="text-white font-medium">${
                  data.backup_count || 0
                }</span>
            </div>
        </div>
    `;
};

// MongoDB Configuration
window.populateMongoDBContent = function (data) {
  const container = document.getElementById("mongodb-content");
  if (!container || !data) return;

  container.innerHTML = `
        <div class="space-y-4">
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Database</span>
                <span class="text-white font-medium">${
                  data.database || "Not set"
                }</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Connection</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium bg-green-100 text-green-800 rounded-full">
                    Connected
                </span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Collections</span>
                <span class="text-white font-medium">Per-kind optimization</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Indexing</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full">
                    Automatic
                </span>
            </div>
        </div>
    `;
};

// Resource Limits Configuration
window.populateResourceLimitsContent = function (data) {
  const container = document.getElementById("resource-limits-content");
  if (!container || !data) return;

  container.innerHTML = `
        <div class="space-y-4">
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Max Connections</span>
                <span class="text-white font-medium">${
                  data.max_connections || "Unlimited"
                }</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Memory Limit</span>
                <span class="text-white font-medium">${
                  data.memory_limit
                    ? window.dashboardUtils.formatBytes(data.memory_limit)
                    : "System default"
                }</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">CPU Cores</span>
                <span class="text-white font-medium">${
                  data.cpu_cores || "All available"
                }</span>
            </div>
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Status</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium bg-green-100 text-green-800 rounded-full">
                    Optimized
                </span>
            </div>
        </div>
    `;
};

// Event Purge Configuration
window.populateEventPurgeContent = function (data) {
  const container = document.getElementById("event-purge-content");
  if (!container || !data) return;

  const purgeEnabled = data.enabled !== false; // Default to enabled if not specified

  container.innerHTML = `
        <div class="space-y-4">
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Purge Enabled</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium ${
                  purgeEnabled
                    ? "bg-green-100 text-green-800"
                    : "bg-red-100 text-red-800"
                } rounded-full">
                    ${purgeEnabled ? "Yes" : "No"}
                </span>
            </div>
            ${
              data.regular_retention_days
                ? `
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Regular Events</span>
                    <span class="text-white font-medium">${data.regular_retention_days} days</span>
                </div>
            `
                : ""
            }
            ${
              data.replaceable_retention_days
                ? `
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Replaceable Events</span>
                    <span class="text-white font-medium">${data.replaceable_retention_days} days</span>
                </div>
            `
                : ""
            }
            ${
              data.ephemeral_retention_hours
                ? `
                <div class="flex justify-between items-center">
                    <span class="text-gray-300">Ephemeral Events</span>
                    <span class="text-white font-medium">${data.ephemeral_retention_hours} hours</span>
                </div>
            `
                : ""
            }
            <div class="flex justify-between items-center">
                <span class="text-gray-300">Schedule</span>
                <span class="text-white font-medium">${
                  data.schedule || "Daily"
                }</span>
            </div>
        </div>
    `;
};

// Console logging for debugging
console.log("Dashboard.js loaded successfully");

// Expose functions globally for Hyperscript to access
window.refreshAllConfigs = function () {
  if (typeof _hyperscript !== "undefined") {
    _hyperscript.evaluate("call refreshAllConfigs()", document.body);
  } else {
    console.warn("Hyperscript not available, using direct function calls");
    // Fallback to direct function calls
    loadAllConfigsDirectly();
  }
};

// Direct loading functions as fallback
function loadAllConfigsDirectly() {
  const endpoints = [
    {
      url: "/api/v1/relay/config/rate_limit",
      handler: window.populateRateLimitTable,
    },
    {
      url: "/api/v1/relay/config/server",
      handler: window.populateServerConfigTable,
    },
    {
      url: "/api/v1/relay/config/auth",
      handler: window.populateAuthConfigTable,
    },
    {
      url: "/api/v1/relay/whitelist",
      handler: window.populateWhitelistContent,
    },
    {
      url: "/api/v1/relay/blacklist",
      handler: window.populateBlacklistContent,
    },
    {
      url: "/api/v1/relay/config/logging",
      handler: window.populateLoggingContent,
    },
    {
      url: "/api/v1/relay/config/mongodb",
      handler: window.populateMongoDBContent,
    },
    {
      url: "/api/v1/relay/config/resource_limits",
      handler: window.populateResourceLimitsContent,
    },
    {
      url: "/api/v1/relay/config/event_purge",
      handler: window.populateEventPurgeContent,
    },
  ];

  endpoints.forEach((endpoint) => {
    fetch(endpoint.url)
      .then((response) => {
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        return response.json();
      })
      .then((data) => {
        endpoint.handler(data);
      })
      .catch((error) => {
        console.error(`Error loading ${endpoint.url}:`, error);
      });
  });

  // Update timestamp
  const timestampEl = document.getElementById("last-updated");
  if (timestampEl) {
    timestampEl.textContent = new Date().toLocaleString();
  }
}

// Auto-refresh every 30 seconds
setInterval(() => {
  if (document.getElementById("rate-limit-table")) {
    console.log("Auto-refreshing dashboard data...");
    loadAllConfigsDirectly();
  }
}, 30000);

// Error handling for failed API calls
window.handleAPIError = function (elementId, errorMessage) {
  const element = document.getElementById(elementId);
  if (element) {
    element.innerHTML = `
            <div class="text-center text-red-400 py-4">
                <p>⚠️ ${errorMessage}</p>
                <button onclick="loadAllConfigsDirectly()" class="mt-2 px-3 py-1 bg-red-600 hover:bg-red-700 rounded text-white text-sm">
                    Retry
                </button>
            </div>
        `;
  }
};

// Initialize dashboard when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", function () {
    console.log("Dashboard initializing...");
    setTimeout(loadAllConfigsDirectly, 100); // Small delay to ensure elements are ready
  });
} else {
  console.log("Dashboard initializing immediately...");
  setTimeout(loadAllConfigsDirectly, 100);
}
