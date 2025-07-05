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
    userProfile: "/api/v1/user/profile",
  },

  // Fetch user profile with caching
  async fetchUserProfile(pubkey) {
    // Check cache first
    if (this.profileCache.has(pubkey)) {
      return this.profileCache.get(pubkey);
    }

    try {
      const response = await fetch(
        `${this.endpoints.userProfile}?pubkey=${pubkey}`
      );
      if (!response.ok) {
        throw new Error(`Profile fetch failed: ${response.status}`);
      }

      const profileData = await response.json();

      // Parse content if it's a JSON string
      let profile = {};
      if (profileData.content) {
        try {
          profile = JSON.parse(profileData.content);
        } catch (e) {
          console.warn(`Failed to parse profile content for ${pubkey}:`, e);
          profile = { about: profileData.content };
        }
      }

      // Cache the parsed profile
      this.profileCache.set(pubkey, profile);
      return profile;
    } catch (error) {
      console.warn(`Failed to fetch profile for ${pubkey}:`, error);
      // Cache empty profile to avoid repeated failed requests
      this.profileCache.set(pubkey, {});
      return {};
    }
  },

  // Create profile card HTML for horizontal layout with source info
  createHorizontalProfileCard(pubkey, profile, source = "direct") {
    const name = profile.name || profile.display_name || "Unknown User";
    const picture = profile.picture || null;

    return `
      <div class="flex-shrink-0 text-center cursor-pointer hover:bg-gray-700 rounded-lg p-2 transition-colors" data-pubkey="${pubkey}">
        <div class="w-16 h-16 mx-auto mb-2">
          ${
            picture
              ? `<img src="${picture}" alt="${name}" class="w-16 h-16 rounded-full object-cover" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
             <div class="w-16 h-16 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-lg" style="display: none;">
               ${name.charAt(0).toUpperCase()}
             </div>`
              : `<div class="w-16 h-16 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-lg">
               ${name.charAt(0).toUpperCase()}
             </div>`
          }
        </div>
        <div class="text-xs text-white font-medium truncate max-w-[80px] mb-1">${name}</div>
        <div class="text-xs ${
          source === "direct" ? "text-green-400" : "text-blue-400"
        } truncate max-w-[80px]">${source}</div>
      </div>
    `;
  },

  // Progressive loading for horizontal key lists with source tracking
  async loadHorizontalKeyProfiles(
    keysWithSources,
    containerId,
    emptyMessage = "No keys found"
  ) {
    const container = document.getElementById(containerId);
    if (!container || !keysWithSources || keysWithSources.length === 0) {
      if (container) {
        container.innerHTML = `<div class="text-center text-gray-400 py-8">${emptyMessage}</div>`;
      }
      return;
    }

    // Show loading state initially with key count
    container.innerHTML = `
      <div class="text-sm text-gray-400 mb-3">Loading ${
        keysWithSources.length
      } profiles...</div>
      <div class="flex space-x-4 overflow-x-auto pb-2">
        ${keysWithSources
          .map(
            () => `
          <div class="flex-shrink-0 text-center animate-pulse">
            <div class="w-16 h-16 bg-gray-600 rounded-full mx-auto mb-2"></div>
            <div class="h-3 bg-gray-600 rounded w-16 mb-1"></div>
            <div class="h-2 bg-gray-600 rounded w-12 mx-auto"></div>
          </div>
        `
          )
          .join("")}
      </div>
    `;

    // Load profiles progressively with small delays
    const profileCards = [];
    for (let i = 0; i < keysWithSources.length; i++) {
      const { pubkey, source } = keysWithSources[i];
      const profile = await this.fetchUserProfile(pubkey);
      profileCards.push(
        this.createHorizontalProfileCard(pubkey, profile, source)
      );

      // Add small delay between requests to be API-friendly
      if (i < keysWithSources.length - 1) {
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    }

    // Update container with horizontal scrolling layout
    container.innerHTML = `
      <div class="text-sm text-gray-400 mb-3">${
        keysWithSources.length
      } users</div>
      <div class="flex space-x-4 overflow-x-auto pb-2 scrollbar-thin scrollbar-thumb-gray-600 scrollbar-track-gray-800">
        ${profileCards.join("")}
      </div>
    `;
  },

  // Profile cache to avoid redundant API calls
  profileCache: new Map(),

  // Initialize dashboard
  init() {
    console.log("New Dashboard initializing...");
    this.updateTimestamp();
    this.setupEventListeners();
    this.refreshAll();
    // No auto-refresh timer - only loads on page load
  },

  // Initialize dashboard
  init() {
    console.log("New Dashboard initializing...");
    this.updateTimestamp();
    this.setupEventListeners();
    this.refreshAll();
    // No auto-refresh timer - only loads on page load
  },

  // Setup event listeners
  setupEventListeners() {
    // No manual refresh button - only auto-refresh on page load
    console.log("Dashboard event listeners initialized (page load only)");
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
    console.log("Loading dashboard data...");

    const loadPromises = [
      // Load whitelist/blacklist first (top priority)
      this.loadWhitelistData(),
      this.loadBlacklistData(),
      // Then other sections
      this.loadRelayOverview(),
      this.loadPolicyLimits(),
      this.loadEventPurgeConfig(),
      this.loadSystemConfig(),
      this.loadUserSyncConfig(),
    ];

    try {
      await Promise.allSettled(loadPromises);
      this.updateTimestamp();
      console.log("Dashboard data loading completed");
    } catch (error) {
      console.error("Dashboard loading error:", error);
    }
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
          <p class="text-sm text-gray-500 mt-2">Refresh the page to retry</p>
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

  // 6. Load Enhanced Whitelist Data with Profiles
  async loadWhitelistData() {
    const [keysData, configData] = await Promise.all([
      this.fetchConfig(this.endpoints.whitelistKeys, "whitelist-config"),
      this.fetchConfig(this.endpoints.whitelistConfig, "whitelist-config"),
    ]);

    if (!keysData || !configData) return;

    // Load configuration section
    const configContainer = document.getElementById("whitelist-config");
    if (configContainer) {
      const totalKeys =
        (keysData.list?.length || 0) +
        (keysData.domains?.reduce(
          (acc, domain) => acc + (domain.pubkeys?.length || 0),
          0
        ) || 0);

      // Calculate total domain keys count
      const totalDomainKeys =
        keysData.domains?.reduce(
          (acc, domain) => acc + (domain.pubkeys?.length || 0),
          0
        ) || 0;

      // Get list of domain names
      const domainNames =
        keysData.domains?.map((domain) => domain.domain) || [];

      configContainer.innerHTML = `
        <div class="space-y-6">
          <!-- Status and Key Counts -->
          <div class="grid grid-cols-3 gap-6">
            <div class="space-y-3">
              <h4 class="text-sm font-medium text-gray-400 uppercase tracking-wide">Status</h4>
              <div class="flex justify-between items-center">
                <span class="text-gray-300">Pubkey Whitelist</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium ${
                  configData.pubkey_whitelist?.enabled
                    ? "bg-green-100 text-green-800"
                    : "bg-gray-100 text-gray-800"
                } rounded-full">
                  ${
                    configData.pubkey_whitelist?.enabled ? "Active" : "Inactive"
                  }
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-300">Domain Whitelist</span>
                <span class="inline-flex px-2 py-1 text-xs font-medium ${
                  configData.domain_whitelist?.enabled
                    ? "bg-purple-100 text-purple-800"
                    : "bg-gray-100 text-gray-800"
                } rounded-full">
                  ${
                    configData.domain_whitelist?.enabled
                      ? "Enabled"
                      : "Disabled"
                  }
                </span>
              </div>
            </div>
            
            <div class="space-y-3">
              <h4 class="text-sm font-medium text-gray-400 uppercase tracking-wide">Key Counts</h4>
              <div class="flex justify-between items-center">
                <span class="text-gray-300">Total Keys</span>
                <span class="text-white font-medium text-lg">${totalKeys}</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-300">Direct Keys</span>
                <span class="text-green-400 font-medium">${
                  keysData.list?.length || 0
                }</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-300">Domain Keys</span>
                <span class="text-blue-400 font-medium">${totalDomainKeys} from ${
        domainNames.length
      } domains</span>
              </div>
            </div>

            <div class="space-y-3">
              ${
                configData.kind_whitelist?.enabled
                  ? `
                <h4 class="text-sm font-medium text-gray-400 uppercase tracking-wide">Allowed Event Kinds</h4>
                <div class="flex flex-wrap gap-1">
                  ${(configData.kind_whitelist?.kinds || [])
                    .map(
                      (kind) => `
                    <span class="inline-flex px-2 py-1 text-xs bg-indigo-900 text-indigo-200 rounded font-mono">${kind}</span>
                  `
                    )
                    .join("")}
                </div>
              `
                  : `
                <h4 class="text-sm font-medium text-gray-400 uppercase tracking-wide">Event Kinds</h4>
                <div class="flex justify-between items-center">
                  <span class="text-gray-300">Kind Whitelist</span>
                  <span class="inline-flex px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded-full">
                    Disabled
                  </span>
                </div>
              `
              }
            </div>
          </div>

          <!-- Domains List -->
          ${
            domainNames.length > 0
              ? `
            <div>
              <h4 class="text-sm font-medium text-gray-400 uppercase tracking-wide mb-3">Whitelisted Domains</h4>
              <div class="flex flex-wrap gap-2">
                ${domainNames
                  .map(
                    (domain) => `
                  <span class="inline-flex px-3 py-1 text-sm bg-blue-900 text-blue-200 rounded-md">${domain}</span>
                `
                  )
                  .join("")}
              </div>
            </div>
          `
              : ""
          }
        </div>
      `;
    }

    // Collect all pubkeys with their sources
    const keysWithSources = [];

    // Add direct list keys
    if (keysData.list) {
      keysData.list.forEach((key) => {
        keysWithSources.push({ pubkey: key, source: "direct" });
      });
    }

    // Add domain keys with domain name as source
    if (keysData.domains) {
      keysData.domains.forEach((domain) => {
        if (domain.pubkeys) {
          domain.pubkeys.forEach((key) => {
            keysWithSources.push({ pubkey: key, source: domain.domain });
          });
        }
      });
    }

    // Load key profiles in horizontal layout with source info
    await this.loadHorizontalKeyProfiles(
      keysWithSources,
      "whitelist-keys",
      "No whitelisted users found"
    );
  },

  // 7. Load Enhanced Blacklist Data with Profiles
  async loadBlacklistData() {
    const [keysData, configData] = await Promise.all([
      this.fetchConfig(this.endpoints.blacklistKeys, "blacklist-config"),
      this.fetchConfig(this.endpoints.blacklistConfig, "blacklist-config"),
    ]);

    if (!keysData || !configData) return;

    // Load configuration section
    const configContainer = document.getElementById("blacklist-config");
    if (configContainer) {
      const permanentCount = keysData.permanent?.length || 0;
      const temporaryCount = keysData.temporary?.length || 0;
      const mutelistCount = Object.keys(keysData.mutelist || {}).length;

      configContainer.innerHTML = `
        <div class="space-y-3">
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Status</span>
            <span class="inline-flex px-2 py-1 text-xs font-medium ${
              configData.enabled
                ? "bg-red-100 text-red-800"
                : "bg-gray-100 text-gray-800"
            } rounded-full">
              ${configData.enabled ? "Active" : "Inactive"}
            </span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Permanent</span>
            <span class="text-white font-medium">${permanentCount}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Temporary</span>
            <span class="text-white font-medium">${temporaryCount}</span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-300">Mute Lists</span>
            <span class="text-white font-medium">${mutelistCount}</span>
          </div>
        </div>
      `;
    }

    // Collect all unique blacklisted pubkeys
    const allPubkeys = new Set();

    // Add permanent blacklist keys
    if (keysData.permanent) {
      keysData.permanent.forEach((key) => allPubkeys.add(key));
    }

    // Add temporary blacklist keys
    if (keysData.temporary) {
      keysData.temporary.forEach((item) => {
        if (typeof item === "string") {
          allPubkeys.add(item);
        } else if (item.pubkey) {
          allPubkeys.add(item.pubkey);
        }
      });
    }

    // Add mutelist keys (limit to first 10 for display purposes)
    if (keysData.mutelist) {
      let count = 0;
      for (const authorKeys of Object.values(keysData.mutelist)) {
        if (Array.isArray(authorKeys)) {
          for (const key of authorKeys) {
            if (count < 10) {
              // Limit display to avoid overwhelming
              allPubkeys.add(key);
              count++;
            } else {
              break;
            }
          }
        }
        if (count >= 10) break;
      }
    }

    // Load key profiles progressively
    await this.loadKeyProfiles(
      Array.from(allPubkeys),
      "blacklist-keys",
      "No blacklisted keys found"
    );
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
