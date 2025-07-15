/**
 * GRAIN Dashboard JavaScript - Phase 1 Redesign
 * Handles the new dashboard structure with focused configuration sections
 */

// New Dashboard Manager with reorganized structure
const dashboardManager = {
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

  // Profile cache to avoid redundant API calls
  profileCache: new Map(),

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
    const name = profile.name || profile.display_name || "?";
    const picture = profile.picture || null;

    return `
    <div class="flex-shrink-0 text-center cursor-pointer hover:bg-gray-700 rounded-lg p-2 transition-colors" data-pubkey="${pubkey}">
      <div class="w-16 h-16 mx-auto mb-2">
        ${
          picture
            ? `<img src="${picture}" alt="${name}" class="w-16 h-16 rounded-full object-cover" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
           <div class="w-16 h-16 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-lg" style="display: none;">
             <img src="https://robohash.org/${pubkey}?set=set6&size=64x64" alt="${name}" class="w-16 h-16 rounded-full object-cover">
           </div>`
            : `<div class="w-16 h-16 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-lg">
             <img src="https://robohash.org/${pubkey}?set=set6&size=64x64" alt="${name}" class="w-16 h-16 rounded-full object-cover">
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
      <div class="flex space-x-4 overflow-x-auto pb-2 custom-scroll">
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

    // Update container with horizontal scrolling layout and custom scroll
    container.innerHTML = `
      <div class="text-sm text-gray-400 mb-3">${
        keysWithSources.length
      } users</div>
      <div class="flex space-x-4 overflow-x-auto pb-2 custom-scroll">
        ${profileCards.join("")}
      </div>
    `;

    // Add horizontal scroll AFTER final content is loaded
    // Small delay to ensure DOM is fully updated
    setTimeout(() => {
      this.enableHorizontalWheelScroll(container);
    }, 50);
  },

  // Initialize dashboard
  init() {
    console.log("New Dashboard initializing...");

    // Fix container width immediately
    const container = document.getElementById("dashboard-main");
    if (container) {
      container.style.width = "100%";
      container.style.maxWidth = "none";
    }

    this.updateTimestamp();
    this.setupEventListeners();
    this.refreshAll();
  },

  // Setup event listeners
  setupEventListeners() {
    // No manual refresh button - only auto-refresh on page load
    console.log("Dashboard event listeners initialized (page load only)");
  },

  enableHorizontalWheelScroll(container) {
    if (!container) return;

    const scrollElement = container.querySelector(".custom-scroll");
    if (!scrollElement) return;

    // Simple wheel to horizontal scroll conversion
    const handleWheel = (e) => {
      // Only if there's horizontal overflow
      if (scrollElement.scrollWidth > scrollElement.clientWidth) {
        e.preventDefault();
        scrollElement.scrollLeft += e.deltaY * 0.5;
      }
    };

    scrollElement.addEventListener("wheel", handleWheel, { passive: false });
    console.log("Horizontal wheel scroll enabled for custom-scroll container");
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
      console.error(`Failed to fetch ${url}:`, error);
      // Show error in container
      const container = document.getElementById(errorContainer);
      if (container) {
        container.innerHTML = `<div class="text-center text-red-400 py-4">Error loading data</div>`;
      }
      return null;
    }
  },

  // 1. Load Relay Overview with 4 containers structure
  async loadRelayOverview() {
    const container = document.getElementById("relay-overview-content");
    if (!container) return;

    // Show loading state
    container.innerHTML = `
    <div class="space-y-3">
      <div class="h-4 bg-gray-700 rounded animate-pulse"></div>
      <div class="w-5/6 h-4 bg-gray-700 rounded animate-pulse"></div>
      <div class="w-4/6 h-4 bg-gray-700 rounded animate-pulse"></div>
    </div>
  `;

    try {
      // Fetch NIP-11 relay information with proper headers
      const response = await fetch(window.location.origin, {
        method: "GET",
        headers: {
          Accept: "application/nostr+json",
          "Content-Type": "application/nostr+json",
        },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch relay info: ${response.status}`);
      }

      const relayInfo = await response.json();

      // Fetch auth and backup relay data for system configuration container
      const [authData, backupData] = await Promise.all([
        this.fetchConfig(this.endpoints.auth, "relay-overview-content"),
        this.fetchConfig(this.endpoints.backupRelay, "relay-overview-content"),
      ]);

      // Create the relay info display with 4 containers
      container.innerHTML = this.createRelayInfoHTML(
        relayInfo,
        authData,
        backupData
      );
    } catch (error) {
      console.error("Failed to load relay information:", error);

      // Show error state with fallback basic info
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
        <div class="text-sm text-gray-400 mt-4 p-3 bg-red-900/20 border border-red-500/30 rounded">
          <p class="text-red-300">⚠️ Unable to load detailed relay information</p>
          <p class="text-xs mt-1">${error.message}</p>
        </div>
      </div>
    `;
    }
  },

  // Helper function to create the relay info HTML with 4 containers
  createRelayInfoHTML(relayInfo, authData, backupData) {
    const {
      name = "🌾 GRAIN Relay",
      description = "Go Relay Architecture for Implementing Nostr",
      banner,
      pubkey,
      contact,
      supported_nips = [],
      software = "https://github.com/0ceanslim/grain",
      version = "Unknown",
      privacy_policy,
      terms_of_service,
      posting_policy,
      tags = [],
    } = relayInfo;

    // Create HTML sections
    let html = '<div class="space-y-6">';

    // Banner if available
    if (banner) {
      html += `
      <div class="rounded-lg overflow-hidden">
        <img src="${banner}" alt="Relay Banner" class="w-full h-32 object-cover">
      </div>
    `;
    }

    // Centered header section with name
    html += `
    <div class="text-center">
      <h3 class="text-2xl font-bold text-white">${this.escapeHtml(name)}</h3>
    </div>
  `;

    // Description
    if (description) {
      html += `
      <div class="bg-gray-750 p-3 rounded-lg">
        <p class="text-white text-sm leading-relaxed text-center">${this.escapeHtml(
          description
        )}</p>
      </div>
    `;
    }

    // Create version link if software is GitHub repo
    let versionDisplay = this.escapeHtml(version);
    if (software.includes("github.com") && version !== "Unknown") {
      const releaseUrl = `${software}/releases/tag/v${version}`;
      versionDisplay = `<a href="${releaseUrl}" target="_blank" class="text-blue-400 hover:text-blue-300">${version}</a>`;
    }

    // 4 Containers Layout
    html += `
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      
      <!-- Left Column -->
      <div class="space-y-6">
        
        <!-- Technical Details Container -->
        <div class="bg-gray-750 border border-gray-600 rounded-lg p-4">
          <h4 class="text-lg font-semibold text-white mb-3 text-center border-b border-gray-600 pb-2">🔧 Technical Details</h4>
          <div class="space-y-3 text-sm">
            <div class="flex justify-between items-center">
              <span class="text-gray-400">Software</span>
              <span class="text-white font-medium">
                ${
                  software.includes("github.com")
                    ? `<a href="${software}" target="_blank" class="text-blue-400 hover:text-blue-300">🌾 GRAIN</a>`
                    : "🌾 GRAIN"
                }
              </span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-400">Version</span>
              <span class="text-white font-medium">${versionDisplay}</span>
            </div>
            
            <!-- Policies Section -->
            <div class="pt-3 border-t border-gray-600">
              <h5 class="text-center text-white font-medium mb-3">Policies</h5>
              ${this.createPoliciesSection(
                privacy_policy,
                terms_of_service,
                posting_policy
              )}
            </div>
          </div>
        </div>

        <!-- Supported NIPs Container -->
        <div class="bg-gray-750 border border-gray-600 rounded-lg p-4">
          <h4 class="text-lg font-semibold text-white mb-3 text-center border-b border-gray-600 pb-2">📋 Supported NIPs</h4>
          <div class="flex flex-wrap gap-1 justify-center">
            ${this.createNIPSLinks(supported_nips)}
          </div>
        </div>

        <!-- Tags Container -->
        <div class="bg-gray-750 border border-gray-600 rounded-lg p-4">
          <h4 class="text-lg font-semibold text-white mb-3 text-center border-b border-gray-600 pb-2">🏷️ Tags</h4>
          <div class="flex flex-wrap gap-1 justify-center">
            ${this.createTagsLinks(tags)}
          </div>
        </div>

      </div>

      <!-- Right Column -->
      <div class="space-y-6">

        <!-- Contact and Admin Container -->
        <div class="bg-gray-750 border border-gray-600 rounded-lg p-4">
          <h4 class="text-lg font-semibold text-white mb-3 text-center border-b border-gray-600 pb-2">📞 Contact & Admin</h4>
          ${this.createAdminSection(pubkey, contact)}
        </div>

        <!-- System Configuration Container -->
        <div class="bg-gray-750 border border-gray-600 rounded-lg p-4">
          <h4 class="text-lg font-semibold text-white mb-3 text-center border-b border-gray-600 pb-2">⚙️ System Configuration</h4>
          <div class="space-y-3 text-sm">
            ${this.createSystemConfigContent(authData, backupData)}
          </div>
        </div>

      </div>

    </div>
  `;

    html += "</div>";

    // Load admin profile after HTML is inserted - THIS IS CRITICAL
    if (pubkey) {
      setTimeout(() => this.loadAdminProfile(pubkey), 100);
    }

    return html;
  },

  // Helper function to create policies section
  createPoliciesSection(privacy_policy, terms_of_service, posting_policy) {
    const policies = [
      { url: privacy_policy, label: "Privacy" },
      { url: terms_of_service, label: "Terms" },
      { url: posting_policy, label: "Posting" },
    ].filter((policy) => policy.url);

    if (policies.length === 0) {
      return `
      <div class="text-center">
        <span class="text-gray-400 text-xs cursor-help" title="No policies configured for this relay">No Policies</span>
      </div>
    `;
    }

    return `
    <div class="flex justify-center space-x-4">
      ${policies
        .map(
          (policy) =>
            `<a href="${policy.url}" target="_blank" class="text-blue-400 hover:text-blue-300 text-xs">${policy.label}</a>`
        )
        .join("")}
    </div>
  `;
  },

  // Helper function to create NIPs links
  createNIPSLinks(supported_nips) {
    if (!supported_nips || supported_nips.length === 0) {
      return `<div class="text-center text-gray-400 text-sm">No NIPs specified</div>`;
    }

    const nipLinks = supported_nips
      .slice(0, 12)
      .map((nip) => {
        const nipNum = String(nip).padStart(2, "0");
        return `<a href="https://github.com/nostr-protocol/nips/blob/master/${nipNum}.md" target="_blank" class="inline-flex px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full hover:bg-blue-200 transition-colors">NIP-${nipNum}</a>`;
      })
      .join("");

    const remaining =
      supported_nips.length > 12
        ? `<span class="text-xs text-gray-400">+${
            supported_nips.length - 12
          } more</span>`
        : "";

    return nipLinks + remaining;
  },

  // Helper function to create tags links
  createTagsLinks(tags) {
    if (!tags || tags.length === 0) {
      return `<div class="text-center text-gray-400 text-sm">No tags specified</div>`;
    }

    const tagLinks = tags
      .slice(0, 8)
      .map(
        (tag) =>
          `<a href="https://nostr.band/?q=%23${encodeURIComponent(
            tag
          )}" target="_blank" class="inline-flex px-2 py-1 text-xs font-medium bg-purple-100 text-purple-800 rounded-full hover:bg-purple-200 transition-colors">${this.escapeHtml(
            tag
          )}</a>`
      )
      .join("");

    const remaining =
      tags.length > 8
        ? `<span class="text-xs text-gray-400">+${tags.length - 8} more</span>`
        : "";

    return tagLinks + remaining;
  },

  // Helper function to create system configuration content
  createSystemConfigContent(authData, backupData) {
    let content = "";

    // Authentication configuration
    if (authData) {
      content += `
      <div class="flex justify-between items-center">
        <span class="text-gray-400">Authentication</span>
        <div class="text-right">
          <div class="inline-flex px-2 py-1 text-xs font-medium ${
            authData.enabled
              ? "bg-green-100 text-green-800"
              : "bg-gray-100 text-gray-800"
          } rounded-full">
            ${authData.enabled ? "Enabled" : "Disabled"}
          </div>
          ${
            authData.enabled && authData.relay_url
              ? `
            <div class="text-xs text-gray-400 mt-1">${this.escapeHtml(
              authData.relay_url
            )}</div>
          `
              : ""
          }
        </div>
      </div>
    `;
    }

    // Backup Relay configuration
    if (backupData) {
      content += `
      <div class="flex justify-between items-center">
        <span class="text-gray-400">Backup Relay</span>
        <div class="text-right">
          <div class="inline-flex px-2 py-1 text-xs font-medium ${
            backupData.enabled
              ? "bg-blue-100 text-blue-800"
              : "bg-gray-100 text-gray-800"
          } rounded-full">
            ${backupData.enabled ? "Enabled" : "Disabled"}
          </div>
          ${
            backupData.enabled && backupData.url
              ? `
            <div class="text-xs text-gray-400 mt-1">${this.escapeHtml(
              backupData.url
            )}</div>
          `
              : ""
          }
        </div>
      </div>
    `;
    }

    // If no data is available
    if (!authData && !backupData) {
      content = `<div class="text-center text-gray-400">Configuration loading...</div>`;
    }

    return content;
  },

  // Helper function to create policy link with warning if missing
  createPolicyLink(url, label, shortLabel) {
    if (url) {
      return `<a href="${url}" target="_blank" class="text-blue-400 hover:text-blue-300 text-sm">${shortLabel}</a>`;
    } else {
      return `<span class="text-gray-500 text-sm cursor-help" title="No ${label.toLowerCase()} specified">⚠️ ${shortLabel}</span>`;
    }
  },

  // Helper function to create admin section with profile
  createAdminSection(pubkey, contact) {
    if (!pubkey && !contact)
      return '<p class="text-gray-400 text-center text-sm">No admin information available</p>';

    return `
    <div class="flex flex-col items-center space-y-2">
      ${
        pubkey
          ? `
        <div id="admin-profile-${pubkey.slice(
          0,
          8
        )}" class="flex flex-col items-center space-y-1">
          <div class="w-12 h-12 bg-gray-600 rounded-full animate-pulse cursor-pointer"></div>
          <span class="text-gray-400 text-xs">Loading...</span>
        </div>
      `
          : ""
      }
      ${this.createContactDisplay(contact)}
    </div>
  `;
  },

  // Helper function to create contact display
  createContactDisplay(contact) {
    if (!contact) return "";

    if (contact.startsWith("mailto:")) {
      return `<a href="${contact}" class="text-blue-400 hover:text-blue-300 text-xs">${contact.replace(
        "mailto:",
        ""
      )}</a>`;
    } else if (
      contact.startsWith("http://") ||
      contact.startsWith("https://")
    ) {
      return `<a href="${contact}" target="_blank" class="text-blue-400 hover:text-blue-300 text-xs">Contact Website</a>`;
    } else {
      return `<span class="text-white text-xs">${this.escapeHtml(
        contact
      )}</span>`;
    }
  },

  // Load admin profile using existing fetchUserProfile function
  async loadAdminProfile(pubkey) {
    const profileContainer = document.getElementById(
      `admin-profile-${pubkey.slice(0, 8)}`
    );
    if (!profileContainer) return;

    try {
      const profile = await this.fetchUserProfile(pubkey);
      const name = profile.name || profile.display_name || "Unknown";
      const picture = profile.picture || null;

      // Convert pubkey to npub
      const npub = await this.convertPubkeyToNpub(pubkey);

      profileContainer.innerHTML = `
      <div class="w-12 h-12 rounded-full overflow-hidden bg-gray-600 flex items-center justify-center cursor-pointer hover:ring-2 hover:ring-blue-400 transition-all" onclick="window.open('https://njump.me/${npub}', '_blank')">
        ${
          picture
            ? `<img src="${picture}" alt="${name}" class="w-full h-full object-cover">`
            : `<span class="text-white text-sm font-bold">${name
                .charAt(0)
                .toUpperCase()}</span>`
        }
      </div>
      <div class="text-center">
        <div class="text-white font-medium text-sm">${this.escapeHtml(
          name
        )}</div>
        <div class="flex items-center space-x-1 text-xs">
          <span class="text-gray-400 font-mono">${npub.slice(
            0,
            12
          )}...${npub.slice(-8)}</span>
          <button onclick="navigator.clipboard.writeText('${npub}'); this.textContent='✓'; setTimeout(() => this.textContent='📋', 1000)" class="text-gray-400 hover:text-white transition-colors" title="Copy npub">📋</button>
        </div>
      </div>
    `;
    } catch (error) {
      console.error("Failed to load admin profile:", error);
      profileContainer.innerHTML = `
      <div class="w-12 h-12 rounded-full bg-gray-600 flex items-center justify-center">
        <span class="text-white text-sm">?</span>
      </div>
      <div class="text-center">
        <div class="text-gray-400 font-mono text-xs">${pubkey.slice(
          0,
          8
        )}...${pubkey.slice(-8)}</div>
      </div>
    `;
    }
  },

  // Helper function to convert pubkey to npub using your API
  async convertPubkeyToNpub(pubkey) {
    try {
      const response = await fetch(
        `/api/v1/convert/pubkey?pubkey=${encodeURIComponent(pubkey)}`
      );

      if (response.ok) {
        const data = await response.json();
        return data.npub || pubkey;
      }
      throw new Error("API conversion failed");
    } catch (error) {
      console.warn(
        "Failed to convert pubkey to npub via API, using fallback:",
        error
      );
      // Fallback: return original pubkey if conversion fails
      return pubkey;
    }
  },

  // Helper function to escape HTML
  escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  },

  // 2. Load Policy & Limits - Redesigned layout with working data loading
  async loadPolicyLimits() {
    try {
      const [rateLimitData, serverData, timeConstraintsData] =
        await Promise.all([
          this.fetchConfig(this.endpoints.rateLimit, "rate-limits-overall"),
          this.fetchConfig(this.endpoints.server, "connection-timeouts"),
          this.fetchConfig(
            this.endpoints.eventTimeConstraints,
            "time-constraints"
          ),
        ]);

      if (!rateLimitData || !serverData || !timeConstraintsData) {
        console.error("Failed to load policy configuration data");
        return;
      }

      // Helper function to format bytes
      const formatBytes = (bytes) => {
        if (bytes === 0) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
      };

      // 1. Populate Rate Limits - Overall
      const rateLimitsOverall = document.getElementById("rate-limits-overall");
      if (rateLimitsOverall) {
        rateLimitsOverall.innerHTML = `
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-700">
                <th class="text-left pb-1 text-xs text-gray-500 font-medium">Type</th>
                <th class="text-right pb-1 text-xs text-gray-500 font-medium">Rate</th>
                <th class="text-right pb-1 text-xs text-gray-500 font-medium">Burst</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-gray-700/30">
                <td class="py-1 text-white text-xs">WebSocket</td>
                <td class="py-1 text-right text-green-400 font-mono text-xs">${rateLimitData.ws_limit}/s</td>
                <td class="py-1 text-right text-gray-300 text-xs">${rateLimitData.ws_burst}</td>
              </tr>
              <tr class="border-b border-gray-700/30">
                <td class="py-1 text-white text-xs">Events</td>
                <td class="py-1 text-right text-green-400 font-mono text-xs">${rateLimitData.event_limit}/s</td>
                <td class="py-1 text-right text-gray-300 text-xs">${rateLimitData.event_burst}</td>
              </tr>
              <tr class="border-b border-gray-700/30">
                <td class="py-1 text-white text-xs">Requests</td>
                <td class="py-1 text-right text-green-400 font-mono text-xs">${rateLimitData.req_limit}/s</td>
                <td class="py-1 text-right text-gray-300 text-xs">${rateLimitData.req_burst}</td>
              </tr>
            </tbody>
          </table>
        `;
      }

      // 2. Populate Rate Limits - By Category
      const rateLimitsCategory = document.getElementById(
        "rate-limits-category"
      );
      if (rateLimitsCategory) {
        if (
          rateLimitData.category_limits &&
          Object.keys(rateLimitData.category_limits).length > 0
        ) {
          const categoryEntries = Object.entries(rateLimitData.category_limits);
          rateLimitsCategory.innerHTML = `
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-700">
                  <th class="text-left pb-1 text-xs text-gray-500 font-medium">Category</th>
                  <th class="text-right pb-1 text-xs text-gray-500 font-medium">Rate</th>
                  <th class="text-right pb-1 text-xs text-gray-500 font-medium">Burst</th>
                </tr>
              </thead>
              <tbody>
                ${categoryEntries
                  .map(
                    ([category, limits]) => `
                  <tr class="border-b border-gray-700/30">
                    <td class="py-1 text-white text-xs capitalize">${category}</td>
                    <td class="py-1 text-right text-green-400 font-mono text-xs">${limits.Limit}/s</td>
                    <td class="py-1 text-right text-gray-300 text-xs">${limits.Burst}</td>
                  </tr>
                `
                  )
                  .join("")}
              </tbody>
            </table>
          `;
        } else {
          rateLimitsCategory.innerHTML =
            '<p class="text-xs text-gray-500">No category-specific limits configured</p>';
        }
      }

      // 3. Populate Rate Limits - By Kind
      const rateLimitsKind = document.getElementById("rate-limits-kind");
      if (rateLimitsKind) {
        if (rateLimitData.kind_limits && rateLimitData.kind_limits.length > 0) {
          rateLimitsKind.innerHTML = `
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-700">
                  <th class="text-left pb-1 text-xs text-gray-500 font-medium">Kind</th>
                  <th class="text-right pb-1 text-xs text-gray-500 font-medium">Rate</th>
                  <th class="text-right pb-1 text-xs text-gray-500 font-medium">Burst</th>
                </tr>
              </thead>
              <tbody>
                ${rateLimitData.kind_limits
                  .map(
                    (kindLimit) => `
                  <tr class="border-b border-gray-700/30">
                    <td class="py-1 text-white text-xs">Kind ${kindLimit.Kind}</td>
                    <td class="py-1 text-right text-green-400 font-mono text-xs">${kindLimit.Limit}/s</td>
                    <td class="py-1 text-right text-gray-300 text-xs">${kindLimit.Burst}</td>
                  </tr>
                `
                  )
                  .join("")}
              </tbody>
            </table>
          `;
        } else {
          rateLimitsKind.innerHTML =
            '<p class="text-xs text-gray-500">No kind-specific limits configured</p>';
        }
      }

      // 4. Populate Size Limits - Overall
      const sizeLimitsOverall = document.getElementById("size-limits-overall");
      if (sizeLimitsOverall) {
        sizeLimitsOverall.innerHTML = `
          <h4 class="text-xs font-medium text-gray-400 uppercase mb-2">Overall</h4>
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-700">
                <th class="text-left pb-1 text-xs text-gray-500 font-medium">Type</th>
                <th class="text-right pb-1 text-xs text-gray-500 font-medium">Limit</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-gray-700/30">
                <td class="py-1 text-white text-xs">Max Event Size</td>
                <td class="py-1 text-right text-purple-400 font-mono text-xs">${formatBytes(
                  rateLimitData.max_event_size
                )}</td>
              </tr>
            </tbody>
          </table>
        `;
      }

      // 5. Populate Size Limits - By Kind
      const sizeLimitsKind = document.getElementById("size-limits-kind");
      if (sizeLimitsKind) {
        if (
          rateLimitData.kind_size_limits &&
          rateLimitData.kind_size_limits.length > 0
        ) {
          sizeLimitsKind.innerHTML = `
            <h4 class="text-xs font-medium text-gray-400 uppercase mb-2">By Kind</h4>
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-700">
                  <th class="text-left pb-1 text-xs text-gray-500 font-medium">Kind</th>
                  <th class="text-right pb-1 text-xs text-gray-500 font-medium">Max Size</th>
                </tr>
              </thead>
              <tbody>
                ${rateLimitData.kind_size_limits
                  .map(
                    (sizeLimit) => `
                  <tr class="border-b border-gray-700/30">
                    <td class="py-1 text-white text-xs">Kind ${
                      sizeLimit.Kind
                    }</td>
                    <td class="py-1 text-right text-purple-400 font-mono text-xs">${formatBytes(
                      sizeLimit.MaxSize
                    )}</td>
                  </tr>
                `
                  )
                  .join("")}
              </tbody>
            </table>
          `;
        } else {
          sizeLimitsKind.innerHTML = `
            <h4 class="text-xs font-medium text-gray-400 uppercase mb-2">By Kind</h4>
            <p class="text-xs text-gray-500">No kind-specific size limits configured</p>
          `;
        }
      }

      // 6. Populate Time Constraints
      const timeConstraints = document.getElementById("time-constraints");
      if (timeConstraints) {
        // Parse future time constraints
        let futureSeconds = 900; // default 15 minutes
        if (
          timeConstraintsData.max_created_at_string &&
          timeConstraintsData.max_created_at_string.startsWith("now+")
        ) {
          const offsetStr = timeConstraintsData.max_created_at_string.replace(
            "now+",
            ""
          );
          if (offsetStr.endsWith("m")) {
            futureSeconds = parseInt(offsetStr.replace("m", "")) * 60;
          } else if (offsetStr.endsWith("s")) {
            futureSeconds = parseInt(offsetStr.replace("s", ""));
          } else if (offsetStr.endsWith("h")) {
            futureSeconds = parseInt(offsetStr.replace("h", "")) * 3600;
          }
        }

        // Parse past time constraints
        const now = Math.floor(Date.now() / 1000);
        const pastSeconds = timeConstraintsData.min_created_at
          ? now - timeConstraintsData.min_created_at
          : 94608000;

        timeConstraints.innerHTML = `
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-700">
                <th class="text-left pb-2 text-xs text-gray-500 font-medium">Type</th>
                <th class="text-right pb-2 text-xs text-gray-500 font-medium">Limit</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Future Events</td>
                <td class="py-2 text-right text-orange-400 font-mono text-xs">${Math.floor(
                  futureSeconds / 60
                )}min</td>
              </tr>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Past Events</td>
                <td class="py-2 text-right text-orange-400 font-mono text-xs">${Math.floor(
                  pastSeconds / 86400
                )} days</td>
              </tr>
            </tbody>
          </table>
        `;
      }

      // 7. Populate Connection Timeouts
      const connectionTimeouts = document.getElementById("connection-timeouts");
      if (connectionTimeouts) {
        connectionTimeouts.innerHTML = `
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-700">
                <th class="text-left pb-2 text-xs text-gray-500 font-medium">Type</th>
                <th class="text-right pb-2 text-xs text-gray-500 font-medium">Value</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Read Timeout</td>
                <td class="py-2 text-right text-blue-400 font-mono text-xs">${serverData.read_timeout}s</td>
              </tr>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Write Timeout</td>
                <td class="py-2 text-right text-blue-400 font-mono text-xs">${serverData.write_timeout}s</td>
              </tr>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Idle Timeout</td>
                <td class="py-2 text-right text-blue-400 font-mono text-xs">${serverData.idle_timeout}s</td>
              </tr>
            </tbody>
          </table>
        `;
      }

      // 8. Populate Connection Limits
      const connectionLimits = document.getElementById("connection-limits");
      if (connectionLimits) {
        connectionLimits.innerHTML = `
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-700">
                <th class="text-left pb-2 text-xs text-gray-500 font-medium">Setting</th>
                <th class="text-right pb-2 text-xs text-gray-500 font-medium">Value</th>
              </tr>
            </thead>
            <tbody>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Max Subscriptions <div class="text-gray-400 text-xs">(per client connection)</div></td>
                <td class="py-2 text-right text-cyan-400 font-mono text-xs">${serverData.max_subscriptions_per_client}</td>
              </tr>
              <tr class="border-b border-gray-700/30">
                <td class="py-2 text-white text-xs">Implicit REQ Limit</td>
                <td class="py-2 text-right text-cyan-400 font-mono text-xs">${serverData.implicit_req_limit}</td>
              </tr>
            </tbody>
          </table>
        `;
      }

      console.log("✅ Policy limits loaded successfully");
    } catch (error) {
      console.error("Error loading policy limits:", error);

      // Show error in all containers
      const containers = [
        "rate-limits-overall",
        "rate-limits-category",
        "rate-limits-kind",
        "size-limits-overall",
        "size-limits-kind",
        "time-constraints",
        "connection-timeouts",
        "connection-limits",
      ];

      containers.forEach((containerId) => {
        const container = document.getElementById(containerId);
        if (container) {
          container.innerHTML = `<div class="text-center text-red-400 py-4">⚠️ Failed to load data</div>`;
        }
      });
    }
  },

  // 3. Load Event Purge Management - Enhanced to match other container styles
  async loadEventPurgeConfig() {
    console.log("🗑️ Loading event purge configuration...");

    const data = await this.fetchConfig(
      this.endpoints.eventPurge,
      "event-purge-content"
    );
    if (!data) return;

    const container = document.getElementById("event-purge-content");
    if (!container) return;

    // Helper function to create category badges
    const createCategoryBadge = (enabled) => {
      return `<span class="inline-flex px-2 py-1 text-xs font-medium ${
        enabled ? "bg-red-100 text-red-800" : "bg-gray-100 text-gray-800"
      } rounded-full">
      ${enabled ? "Purging" : "Keeping"}
    </span>`;
    };

    // Format interval display
    const formatInterval = (hours) => {
      if (hours < 24) {
        return `${hours} hour${hours !== 1 ? "s" : ""}`;
      } else {
        const days = Math.floor(hours / 24);
        const remainingHours = hours % 24;
        if (remainingHours === 0) {
          return `${days} day${days !== 1 ? "s" : ""}`;
        } else {
          return `${days}d ${remainingHours}h`;
        }
      }
    };

    const formatPurgeInterval = (minutes) => {
      if (minutes < 60) {
        return `${minutes} minute${minutes !== 1 ? "s" : ""}`;
      } else {
        const hours = Math.floor(minutes / 60);
        const remainingMinutes = minutes % 60;
        if (remainingMinutes === 0) {
          return `${hours} hour${hours !== 1 ? "s" : ""}`;
        } else {
          return `${hours}h ${remainingMinutes}m`;
        }
      }
    };

    container.innerHTML = `
    <div class="space-y-6">
      <!-- Main Status Section -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <!-- Status Column -->
        <div class="bg-gray-750 rounded-lg p-4 border border-gray-600 space-y-4">
          <!-- Primary Status -->
          <div class="flex justify-between items-center">
            <span class="text-sm font-medium text-gray-300">Status</span>
            <span class="inline-flex px-3 py-1 text-sm font-medium ${
              data.enabled
                ? "bg-green-100 text-green-800"
                : "bg-red-100 text-red-800"
            } rounded-full">
              ${data.enabled ? "Enabled" : "Disabled"}
            </span>
          </div>
          <div class="text-xs text-gray-400 mb-3">
            ${
              data.enabled
                ? "Events are being purged automatically"
                : "Event purging is disabled"
            }
          </div>
          
          <!-- Whitelist Protection -->
          <div class="pt-3 border-t border-gray-600">
            <div class="flex justify-between items-center">
              <span class="text-sm font-medium text-gray-300">Whitelist Protection</span>
              <span class="inline-flex px-3 py-1 text-sm font-medium ${
                data.exclude_whitelisted
                  ? "bg-green-100 text-green-800"
                  : "bg-yellow-100 text-yellow-800"
              } rounded-full">
                ${data.exclude_whitelisted ? "Protected" : "Unprotected"}
              </span>
            </div>
            <div class="text-xs text-gray-400 mt-2">
              ${
                data.exclude_whitelisted
                  ? "Whitelisted users' events are safe"
                  : "Whitelisted users' events can be purged"
              }
            </div>
          </div>
        </div>

        <!-- Timing Configuration -->
        <div class="bg-gray-750 rounded-lg p-4 border border-gray-600">
          <h4 class="text-sm font-medium text-white mb-4">⏰ Timing Configuration</h4>
          <div class="space-y-4">
            <div class="flex justify-between items-center">
              <div>
                <span class="text-gray-300 text-sm">Event Retention</span>
                <div class="text-xs text-gray-400">How long to keep events</div>
              </div>
              <span class="text-white font-medium text-lg">${formatInterval(
                data.keep_interval_hours
              )}</span>
            </div>
            <div class="flex justify-between items-center">
              <div>
                <span class="text-gray-300 text-sm">Purge Frequency</span>
                <div class="text-xs text-gray-400">How often to clean up</div>
              </div>
              <span class="text-white font-medium text-lg">${formatPurgeInterval(
                data.purge_interval_minutes
              )}</span>
            </div>
          </div>
        </div>
      </div>

      ${
        data.enabled
          ? `
        <!-- Event Category Configuration -->
        <div class="bg-gray-750 rounded-lg border border-gray-600">
          <div class="px-4 py-3 border-b border-gray-600">
            <h4 class="text-sm font-medium text-white">📂 Event Categories</h4>
          </div>
          <div class="p-4">
            <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
              ${Object.entries(data.purge_by_category || {})
                .map(
                  ([category, enabled]) => `
                <div class="flex flex-col items-center space-y-2 p-3 rounded-lg ${
                  enabled
                    ? "bg-red-900/20 border border-red-700"
                    : "bg-gray-700/30 border border-gray-600"
                }">
                  <span class="text-sm font-medium text-white capitalize">${category}</span>
                  ${createCategoryBadge(enabled)}
                </div>
              `
                )
                .join("")}
            </div>
          </div>
        </div>

        ${
          data.purge_by_kind_enabled
            ? `
          <!-- Event Kinds Configuration -->
          <div class="bg-gray-750 rounded-lg border border-gray-600">
            <div class="px-4 py-3 border-b border-gray-600">
              <h4 class="text-sm font-medium text-white">🏷️ Event Kinds to Purge</h4>
            </div>
            <div class="p-4">
              ${
                data.kinds_to_purge && data.kinds_to_purge.length > 0
                  ? `
                <div class="flex flex-wrap gap-2">
                  ${data.kinds_to_purge
                    .map(
                      (kind) => `
                    <span class="inline-flex px-3 py-1 text-sm font-medium bg-red-100 text-red-800 rounded-full">
                      Kind ${kind}
                    </span>
                  `
                    )
                    .join("")}
                </div>
                <div class="mt-3 text-xs text-gray-400">
                  Only these event kinds will be purged when kind-specific purging is enabled
                </div>
              `
                  : `
                <div class="text-center text-gray-400 py-4">
                  <span class="text-sm">No specific kinds configured for purging</span>
                </div>
              `
              }
            </div>
          </div>
        `
            : `
          <!-- Kind Purging Disabled Notice -->
          <div class="bg-gray-750 rounded-lg border border-gray-600">
            <div class="px-4 py-3 border-b border-gray-600">
              <h4 class="text-sm font-medium text-white">🏷️ Event Kinds</h4>
            </div>
            <div class="p-4">
              <div class="text-center text-gray-400 py-4">
                <span class="text-sm">Kind-specific purging is disabled</span>
                <div class="text-xs mt-1">All event categories follow the category rules above</div>
              </div>
            </div>
          </div>
        `
        }
      `
          : `
        <!-- Disabled State Information -->
        <div class="bg-gray-750 rounded-lg border border-gray-600">
          <div class="p-6 text-center">
            <div class="text-gray-400 mb-4">
              <svg class="w-12 h-12 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"></path>
              </svg>
            </div>
            <h3 class="text-white font-medium mb-2">Event Purging Disabled</h3>
            <p class="text-sm text-gray-400 mb-4">
              Event purging is currently disabled. Events will be stored indefinitely until purging is enabled.
            </p>
            <div class="text-center max-w-md mx-auto">
              <div class="text-xs">
                <span class="text-gray-300 font-medium">Whitelist Protection:</span>
                <span class="text-gray-400 ml-2">${
                  data.exclude_whitelisted ? "Enabled" : "Disabled"
                }</span>
              </div>
            </div>
          </div>
        </div>
      `
      }
    </div>
  `;

    console.log("✅ Event purge configuration loaded successfully");
  },

  // 4. Load User Sync Configuration (experimental section)
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
        <div class="space-y-4">
          <!-- Centered Two Column Layout -->
          <div class="max-w-4xl mx-auto">
            <div class="whitelist-config-grid">
              <!-- Status Column (with Event Kinds) -->
              <div class="space-y-3 text-center">
                <h4 class="text-xs font-medium text-gray-400 uppercase tracking-wide">Status</h4>
                <div class="space-y-3">
                  <!-- Whitelist Status -->
                  <div class="space-y-2">
                    <div class="flex flex-col items-center space-y-1">
                      <span class="text-sm text-gray-300">Pubkey Whitelist</span>
                      <span class="inline-flex px-2 py-1 text-xs font-medium ${
                        configData.pubkey_whitelist?.enabled
                          ? "bg-green-100 text-green-800"
                          : "bg-gray-100 text-gray-800"
                      } rounded-full">
                        ${
                          configData.pubkey_whitelist?.enabled
                            ? "Active"
                            : "Inactive"
                        }
                      </span>
                    </div>
                    <div class="flex flex-col items-center space-y-1">
                      <span class="text-sm text-gray-300">Domain Whitelist</span>
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
      
                  <!-- Event Kinds in Status Column -->
                  <div class="pt-2 border-t border-gray-600">
                    ${
                      configData.kind_whitelist?.enabled
                        ? `
                      <div class="flex flex-col items-center space-y-2">
                        <span class="text-xs font-medium text-gray-400 uppercase tracking-wide">Allowed Event Kinds</span>
                        <div class="flex flex-wrap gap-1 justify-center max-w-[200px]">
                          ${(configData.kind_whitelist?.kinds || [])
                            .map(
                              (kind) => `
                            <span class="inline-flex px-2 py-1 text-xs bg-indigo-900 text-indigo-200 rounded font-mono">${kind}</span>
                          `
                            )
                            .join("")}
                        </div>
                      </div>
                      `
                        : `
                      <div class="flex flex-col items-center space-y-1">
                        <span class="text-xs font-medium text-gray-400 uppercase tracking-wide">Allowed Event Kinds</span>
                        <span class="inline-flex px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded">
                          Any
                        </span>
                      </div>
                      `
                    }
                  </div>
                </div>
              </div>
      
              <!-- Key Counts Column -->
              <div class="space-y-3 text-center">
                <h4 class="text-xs font-medium text-gray-400 uppercase tracking-wide">Key Counts</h4>
                <div class="space-y-2">
                  <div class="flex flex-col items-center space-y-1">
                    <span class="text-sm text-gray-300">Total Keys</span>
                    <span class="text-white font-medium text-lg">${totalKeys}</span>
                  </div>
                  <div class="flex flex-col items-center space-y-1">
                    <span class="text-sm text-gray-300">Direct Keys</span>
                    <span class="text-green-400 font-medium text-sm">${
                      keysData.list?.length || 0
                    }</span>
                  </div>
                  <div class="flex flex-col items-center space-y-1">
                    <span class="text-sm text-gray-300">Domain Keys</span>
                    <span class="text-blue-400 font-medium text-sm">${totalDomainKeys} from ${
        domainNames.length
      } domains</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
      
          <!-- Domains List (centered with better spacing) -->
          ${
            domainNames.length > 0
              ? `
          <div class="pt-4 border-t border-gray-600">
            <div class="max-w-3xl mx-auto text-center">
              <h4 class="text-xs font-medium text-gray-400 uppercase tracking-wide mb-3">Whitelisted Domains</h4>
              <div class="flex flex-wrap gap-3 justify-center">
                ${domainNames
                  .map(
                    (domain) => `
                  <a href="https://${domain}" target="_blank" rel="noopener noreferrer" class="inline-flex px-4 py-2 text-sm bg-blue-900 text-blue-200 rounded-lg border border-blue-700 hover:bg-blue-800 transition-colors cursor-pointer">${domain}</a>
                `
                  )
                  .join("")}
              </div>
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

  // 7. Load Enhanced Blacklist Data with Profiles - Restructured
  async loadBlacklistData() {
    const [keysData, configData] = await Promise.all([
      this.fetchConfig(this.endpoints.blacklistKeys, "blacklist-config"),
      this.fetchConfig(this.endpoints.blacklistConfig, "blacklist-config"),
    ]);

    if (!keysData || !configData) return;

    // Load configuration section with new structure
    const configContainer = document.getElementById("blacklist-config");
    if (configContainer) {
      const permanentCount = keysData.permanent?.length || 0;
      const temporaryCount = keysData.temporary?.length || 0;

      // Count total mutelist entries across all authors
      const mutelistTotalCount = Object.values(keysData.mutelist || {}).reduce(
        (total, entries) => total + entries.length,
        0
      );

      // Count mutelist authors
      const mutelistAuthorCount = Object.keys(keysData.mutelist || {}).length;

      configContainer.innerHTML = `
        <div class="space-y-4">
          <!-- Status Section -->
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
          
          <!-- Expandable Config Section -->
          <div class="border-t border-gray-600 pt-3">
            <button 
              class="w-full flex justify-between items-center text-sm text-gray-300 hover:text-white transition-colors"
              onclick="this.nextElementSibling.classList.toggle('hidden')"
            >
              <span>Configuration Details</span>
              <svg class="w-4 h-4 transform transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
              </svg>
            </button>
            <div class="mt-2 space-y-2 text-xs hidden">
              <div class="flex justify-between items-center">
                <span class="text-gray-400">Max Temp Bans</span>
                <span class="text-yellow-400">${
                  configData.max_temp_bans || 0
                }</span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-400">Temp Ban Duration</span>
                <span class="text-yellow-400">${Math.floor(
                  (configData.temp_ban_duration || 0) / 3600
                )}h</span>
              </div>
              ${
                configData.temp_ban_words &&
                configData.temp_ban_words.length > 0
                  ? `
                <div class="pt-2 border-t border-gray-700">
                  <span class="text-gray-400 block mb-1">Temp Ban Words</span>
                  <div class="flex flex-wrap gap-1">
                    ${configData.temp_ban_words
                      .map(
                        (word) =>
                          `<span class="px-2 py-1 bg-yellow-900 text-yellow-200 rounded text-xs">${word}</span>`
                      )
                      .join("")}
                  </div>
                </div>
              `
                  : ""
              }
              <div class="pt-2 border-t border-gray-700 text-xs text-gray-500">
                <span>⚠️ Permanent ban words not shown as they may be offensive</span>
              </div>
            </div>
          </div>

          <!-- User Counts Summary -->
          <div class="grid grid-cols-3 gap-3 pt-3 border-t border-gray-600">
            <div class="text-center">
              <div class="text-lg font-medium text-red-400">${permanentCount}</div>
              <div class="text-xs text-gray-400">Permanent</div>
            </div>
            <div class="text-center">
              <div class="text-lg font-medium text-yellow-400">${temporaryCount}</div>
              <div class="text-xs text-gray-400">Temporary</div>
            </div>
            <div class="text-center">
              <div class="text-lg font-medium text-orange-400">${mutelistTotalCount}</div>
              <div class="text-xs text-gray-400">Mutelist</div>
            </div>
          </div>

          <!-- Mutelist Authors Section -->
          ${
            mutelistAuthorCount > 0
              ? `
            <div class="pt-3 border-t border-gray-600">
              <h4 class="text-sm text-gray-300 mb-2">Mutelist Authors (${mutelistAuthorCount})</h4>
              <div id="mutelist-authors" class="flex space-x-3 overflow-x-auto pb-2 custom-scroll">
                <div class="text-xs text-gray-400">Loading authors...</div>
              </div>
            </div>
          `
              : ""
          }
        </div>
      `;

      // Load mutelist author profiles if any exist
      if (mutelistAuthorCount > 0) {
        this.loadMutelistAuthors(Object.keys(keysData.mutelist));
      }
    }

    // Collect all blacklisted keys with their sources (enhanced)
    const keysWithSources = [];

    // Add permanent blacklist keys
    if (keysData.permanent) {
      keysData.permanent.forEach((key) => {
        keysWithSources.push({
          pubkey: key,
          source: "permanent",
          sourceType: "permanent",
        });
      });
    }

    // Add temporary blacklist keys
    if (keysData.temporary) {
      keysData.temporary.forEach((entry) => {
        // Handle both old format (string) and new format (object with expiration)
        const pubkey = typeof entry === "string" ? entry : entry.pubkey;
        keysWithSources.push({
          pubkey: pubkey,
          source: "temporary",
          sourceType: "temporary",
        });
      });
    }

    // Add mutelist keys with author attribution
    if (keysData.mutelist) {
      Object.entries(keysData.mutelist).forEach(
        ([authorPubkey, mutedPubkeys]) => {
          mutedPubkeys.forEach((mutedPubkey) => {
            keysWithSources.push({
              pubkey: mutedPubkey,
              source: authorPubkey,
              sourceType: "mutelist",
              authorPubkey: authorPubkey,
            });
          });
        }
      );
    }

    // Load user profiles with enhanced source info
    await this.loadEnhancedBlacklistProfiles(
      keysWithSources,
      "blacklist-keys",
      "No blacklisted users found"
    );
  },

  // Load mutelist author profiles separately
  async loadMutelistAuthors(authorPubkeys) {
    const container = document.getElementById("mutelist-authors");
    if (!container || !authorPubkeys || authorPubkeys.length === 0) return;

    // Show loading state
    container.innerHTML = authorPubkeys
      .map(
        () => `
      <div class="flex-shrink-0 text-center animate-pulse">
        <div class="w-12 h-12 bg-gray-600 rounded-full mx-auto mb-1"></div>
        <div class="h-2 bg-gray-600 rounded w-12"></div>
      </div>
    `
      )
      .join("");

    // Load author profiles
    const authorCards = [];
    for (const authorPubkey of authorPubkeys) {
      const profile = await this.fetchUserProfile(authorPubkey);
      const name = profile.name || profile.display_name || "?";
      const picture = profile.picture || null;

      authorCards.push(`
        <div class="flex-shrink-0 text-center cursor-pointer hover:bg-gray-700 rounded-lg p-2 transition-colors" data-pubkey="${authorPubkey}">
          <div class="w-12 h-12 mx-auto mb-1">
            ${
              picture
                ? `<img src="${picture}" alt="${name}" class="w-12 h-12 rounded-full object-cover" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
                <div class="w-12 h-12 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-sm" style="display: none;">
                  <img src="https://robohash.org/${authorPubkey}?set=set6&size=48x48" alt="${name}" class="w-12 h-12 rounded-full object-cover">
                </div>`
                : `<div class="w-12 h-12 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-sm">
                  <img src="https://robohash.org/${authorPubkey}?set=set6&size=48x48" alt="${name}" class="w-12 h-12 rounded-full object-cover">
                </div>`
            }
          </div>
          <div class="text-xs text-white font-medium truncate max-w-[60px]">${name}</div>
        </div>
      `);

      // Small delay between requests
      if (authorPubkey !== authorPubkeys[authorPubkeys.length - 1]) {
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    }

    container.innerHTML = authorCards.join("");

    // Enable horizontal scroll for authors
    setTimeout(() => {
      this.enableHorizontalWheelScroll(container.parentElement);
    }, 50);
  },

  // Enhanced profile loading for blacklisted users with better source attribution
  async loadEnhancedBlacklistProfiles(
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

    // Show loading state
    container.innerHTML = `
      <div class="text-sm text-gray-400 mb-3">Loading ${
        keysWithSources.length
      } profiles...</div>
      <div class="flex space-x-4 overflow-x-auto pb-2 custom-scroll">
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

    // Load profiles progressively
    const profileCards = [];
    for (let i = 0; i < keysWithSources.length; i++) {
      const { pubkey, source, sourceType, authorPubkey } = keysWithSources[i];
      const profile = await this.fetchUserProfile(pubkey);

      // Get author name for mutelist entries
      let displaySource = source;
      if (sourceType === "mutelist" && authorPubkey) {
        const authorProfile = await this.fetchUserProfile(authorPubkey);
        displaySource =
          authorProfile.name || authorProfile.display_name || "Mutelist";
      }

      profileCards.push(
        this.createEnhancedBlacklistProfileCard(
          pubkey,
          profile,
          displaySource,
          sourceType
        )
      );

      // Small delay between requests
      if (i < keysWithSources.length - 1) {
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    }

    // Update container with final content
    container.innerHTML = `
      <div class="text-sm text-gray-400 mb-3">${
        keysWithSources.length
      } users</div>
      <div class="flex space-x-4 overflow-x-auto pb-2 custom-scroll">
        ${profileCards.join("")}
      </div>
    `;

    // Enable horizontal scroll
    setTimeout(() => {
      this.enableHorizontalWheelScroll(container);
    }, 50);
  },

  // Enhanced profile card with better source indication
  createEnhancedBlacklistProfileCard(pubkey, profile, source, sourceType) {
    const name = profile.name || profile.display_name || "?";
    const picture = profile.picture || null;

    // Source styling based on type
    let sourceClass, sourceLabel;
    switch (sourceType) {
      case "permanent":
        sourceClass = "text-red-400";
        sourceLabel = "Permanent";
        break;
      case "temporary":
        sourceClass = "text-yellow-400";
        sourceLabel = "Temporary";
        break;
      case "mutelist":
        sourceClass = "text-orange-400";
        sourceLabel = source; // Author name or "Mutelist"
        break;
      default:
        sourceClass = "text-gray-400";
        sourceLabel = source;
    }

    return `
      <div class="flex-shrink-0 text-center cursor-pointer hover:bg-gray-700 rounded-lg p-2 transition-colors" data-pubkey="${pubkey}">
        <div class="w-16 h-16 mx-auto mb-2">
          ${
            picture
              ? `<img src="${picture}" alt="${name}" class="w-16 h-16 rounded-full object-cover" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
              <div class="w-16 h-16 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-lg" style="display: none;">
                <img src="https://robohash.org/${pubkey}?set=set6&size=64x64" alt="${name}" class="w-16 h-16 rounded-full object-cover">
              </div>`
              : `<div class="w-16 h-16 bg-gray-600 rounded-full flex items-center justify-center text-white font-medium text-lg">
                <img src="https://robohash.org/${pubkey}?set=set6&size=64x64" alt="${name}" class="w-16 h-16 rounded-full object-cover">
              </div>`
          }
        </div>
        <div class="text-xs text-white font-medium truncate max-w-[80px] mb-1">${name}</div>
        <div class="text-xs ${sourceClass} truncate max-w-[80px]">${sourceLabel}</div>
      </div>
    `;
  },

  // Utility functions for status badges
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
    dashboardManager.init();
  });
} else {
  dashboardManager.init();
}

// Function to update relay status based on whitelist configuration
async function updateRelayStatus() {
  console.log("Updating relay status...");

  try {
    console.log(
      "Fetching whitelist config from /api/v1/relay/config/whitelist"
    );
    const response = await fetch("/api/v1/relay/config/whitelist");

    console.log("Response status:", response.status);

    if (!response.ok) {
      throw new Error(`Failed to fetch whitelist config: ${response.status}`);
    }

    const config = await response.json();
    console.log("Whitelist config received:", config);

    const statusElement = document.getElementById("relay-status");
    console.log("Status element found:", !!statusElement);

    if (statusElement) {
      const isPrivate = config.pubkey_whitelist?.enabled || false;
      console.log("Is private relay:", isPrivate);

      if (isPrivate) {
        // Private relay - orange/amber colors
        statusElement.innerHTML = `
          <div class="w-3 h-3 bg-amber-500 rounded-full animate-pulse"></div>
          <span class="font-medium text-amber-400">Private Relay Online</span>
        `;
        console.log("Updated to private relay status");
      } else {
        // Public relay - green colors
        statusElement.innerHTML = `
          <div class="w-3 h-3 bg-green-500 rounded-full animate-pulse"></div>
          <span class="font-medium text-green-400">Public Relay Online</span>
        `;
        console.log("Updated to public relay status");
      }
    } else {
      console.error("relay-status element not found");
    }
  } catch (error) {
    console.error("Failed to update relay status:", error);
    // On error, show neutral status
    const statusElement = document.getElementById("relay-status");
    if (statusElement) {
      statusElement.innerHTML = `
        <div class="w-3 h-3 bg-gray-500 rounded-full animate-pulse"></div>
        <span class="font-medium text-gray-400">Relay Online</span>
      `;
    }
  }
}

// Call the function when page loads normally
document.addEventListener("DOMContentLoaded", function () {
  console.log("DOM loaded, calling updateRelayStatus");
  updateRelayStatus();
});

// Call the function when HTMX loads content (for navigation)
document.addEventListener("htmx:afterSwap", function (event) {
  // Only update if we're loading the home page
  if (event.detail.target.id === "main-content") {
    console.log("HTMX content swapped, calling updateRelayStatus");
    setTimeout(updateRelayStatus, 100); // Small delay to ensure elements are ready
  }
});

// Also try calling it after a short delay in case of timing issues
setTimeout(updateRelayStatus, 1000);

// Expose globally for Hyperscript and compatibility
window.dashboardManager = dashboardManager;

console.log("New Dashboard.js loaded successfully");
