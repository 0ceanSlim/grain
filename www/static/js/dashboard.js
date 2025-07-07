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
      console.error(`Failed to fetch ${url}:`, error);
      // Show error in container
      const container = document.getElementById(errorContainer);
      if (container) {
        container.innerHTML = `<div class="text-center text-red-400 py-4">Error loading data</div>`;
      }
      return null;
    }
  },

  // 1. Load Relay Overview (fetches NIP-11 data from relay)
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

      // Create the relay info display
      container.innerHTML = this.createRelayInfoHTML(relayInfo);
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

  // Helper function to create the relay info HTML
  createRelayInfoHTML(relayInfo) {
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
    let html = '<div class="space-y-4">';

    // Banner if available - fills the top of the container
    if (banner) {
      html += `
    <div class="rounded-lg overflow-hidden">
      <img src="${banner}" alt="Relay Banner" class="w-full h-32 object-cover">
    </div>
  `;
    }

    // Centered header section with name and icon
    html += `
  <div class="text-center">
        <h3 class="text-2xl font-bold text-white">${this.escapeHtml(name)}</h3>
  </div>
`;

    // Description (more compact)
    if (description) {
      html += `
    <div class="bg-gray-750 p-2 rounded-lg">
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

    // Compact Technical Information Grid with integrated policies
    html += `
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="bg-gray-750 p-3 rounded-lg">
        <h4 class="text-sm font-medium text-white mb-3 text-center">Technical Details</h4>
        <div class="space-y-2 text-sm">
          <div class="flex justify-between items-center">
            <span class="text-gray-400">Software</span>
            <span class="text-white font-medium">
              ${
                software.includes("github.com")
                  ? `<a href="${software}" target="_blank" class="text-blue-400 hover:text-blue-300">🌾 GRAIN</a>`
                  : this.escapeHtml(software)
              }
            </span>
          </div>
          <div class="flex justify-between items-center">
            <span class="text-gray-400">Version</span>
            <span class="text-white font-medium">${versionDisplay}</span>
          </div>
        </div>
        
        <!-- Policies Section -->
        <h5 class="text-sm font-medium text-white mt-4 mb-2 text-center">Policies</h5>
        <div class="flex justify-center gap-3 text-sm">
          ${this.createPolicyLink(privacy_policy, "Privacy Policy", "Privacy")}
          ${this.createPolicyLink(
            terms_of_service,
            "Terms of Service",
            "Terms"
          )}
          ${this.createPolicyLink(posting_policy, "Posting Policy", "Posting")}
        </div>
      </div>
      
      <div class="bg-gray-750 p-3 rounded-lg">
        <h4 class="text-sm font-medium text-white mb-3 text-center">Contact & Admin</h4>
        ${this.createAdminSection(pubkey, contact)}
      </div>
    </div>
    `;

    // Supported NIPs section (more compact)
    if (supported_nips && supported_nips.length > 0) {
      html += `
    <div class="bg-gray-750 p-3 rounded-lg">
      <h4 class="text-sm font-medium text-white mb-3 text-center">Supported NIPs</h4>
      <div class="flex flex-wrap gap-1.5 justify-center">
        ${supported_nips
          .map(
            (nip) =>
              `<a href="https://github.com/nostr-protocol/nips/blob/master/${String(
                nip
              ).padStart(
                2,
                "0"
              )}.md" target="_blank" class="inline-flex items-center px-2 py-1 text-xs font-mono bg-blue-500/20 text-blue-300 border border-blue-500/30 rounded hover:bg-blue-500/30 transition-colors">${nip}</a>`
          )
          .join("")}
      </div>
    </div>
  `;
    }

    // Tags section (more compact)
    if (tags && tags.length > 0) {
      html += `
    <div class="bg-gray-750 p-3 rounded-lg">
      <h4 class="text-sm font-medium text-white mb-3 text-center">Tags</h4>
      <div class="flex flex-wrap gap-1.5 justify-center">
        ${tags
          .map(
            (tag) =>
              `<span class="inline-flex items-center px-2 py-1 text-xs bg-purple-500/20 text-purple-300 border border-purple-500/30 rounded">${this.escapeHtml(
                tag
              )}</span>`
          )
          .join("")}
      </div>
    </div>
  `;
    }

    html += "</div>";

    // Load admin profile after HTML is inserted
    if (pubkey) {
      setTimeout(() => this.loadAdminProfile(pubkey), 100);
    }

    return html;
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
      const response = await fetch("/api/v1/convert/pubkey", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          pubkey: pubkey,
        }),
      });

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

  // 2. Load Policy & Limits (rate limits + timeouts + time constraints)
  // Fixed loadPolicyLimits function with formatBytes included
  async loadPolicyLimits() {
    try {
      const [rateLimitData, serverData, timeConstraintsData] =
        await Promise.all([
          this.fetchConfig(this.endpoints.rateLimit, "policy-limits-table"),
          this.fetchConfig(this.endpoints.server, "policy-limits-table"),
          this.fetchConfig(
            this.endpoints.eventTimeConstraints,
            "policy-limits-table"
          ),
        ]);

      if (!rateLimitData || !serverData || !timeConstraintsData) {
        console.error("Failed to load policy configuration data");
        return;
      }

      const tbody = document.getElementById("policy-limits-table");
      if (!tbody) {
        console.error("Policy limits table not found");
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

      const rows = [];

      // === EVENT TIME CONSTRAINTS SECTION ===
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

      const now = Math.floor(Date.now() / 1000);
      const pastSeconds = timeConstraintsData.min_created_at
        ? now - timeConstraintsData.min_created_at
        : 94608000;

      rows.push({
        type: "Future Events",
        limit: `${futureSeconds}s`,
        burst: "Max future time",
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Past Events",
        limit: `${Math.floor(pastSeconds / 86400)} days`,
        burst: "Max past time",
        status: this.getStatusBadge(true),
      });

      // === SERVER CONFIGURATION SECTION ===
      rows.push({
        type: "Read Timeout",
        limit: `${serverData.read_timeout}s`,
        burst: "HTTP requests",
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Write Timeout",
        limit: `${serverData.write_timeout}s`,
        burst: "HTTP responses",
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Idle Timeout",
        limit: `${serverData.idle_timeout}s`,
        burst: "Connections",
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Max Subscriptions",
        limit: `${serverData.max_subscriptions_per_client}`,
        burst: "Per client",
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Implicit REQ Limit",
        limit: `${serverData.implicit_req_limit}`,
        burst: "Query results",
        status: this.getStatusBadge(true),
      });

      // === RATE LIMITING SECTION ===
      rows.push({
        type: "WebSocket Messages",
        limit: `${rateLimitData.ws_limit}/sec`,
        burst: `${rateLimitData.ws_burst} messages`,
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Event Publishing",
        limit: `${rateLimitData.event_limit}/sec`,
        burst: `${rateLimitData.event_burst} events`,
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Query Requests",
        limit: `${rateLimitData.req_limit}/sec`,
        burst: `${rateLimitData.req_burst} queries`,
        status: this.getStatusBadge(true),
      });

      rows.push({
        type: "Max Event Size",
        limit: formatBytes(rateLimitData.max_event_size),
        burst: "Per event",
        status: this.getStatusBadge(true),
      });

      // === KIND-SPECIFIC LIMITS (if configured) ===
      if (rateLimitData.kind_limits && rateLimitData.kind_limits.length > 0) {
        rateLimitData.kind_limits.forEach((kindLimit) => {
          rows.push({
            type: `Kind ${kindLimit.Kind}`,
            limit: `${kindLimit.Limit}/sec`,
            burst: `${kindLimit.Burst} events`,
            status: this.getStatusBadge(true),
          });
        });
      }

      // === KIND SIZE LIMITS (if configured) ===
      if (
        rateLimitData.kind_size_limits &&
        rateLimitData.kind_size_limits.length > 0
      ) {
        rateLimitData.kind_size_limits.forEach((sizeLimit) => {
          rows.push({
            type: `Kind ${sizeLimit.Kind} Size`,
            limit: formatBytes(sizeLimit.MaxSize),
            burst: "Max size",
            status: this.getStatusBadge(true),
          });
        });
      }

      // === CATEGORY LIMITS (if configured) ===
      if (rateLimitData.category_limits) {
        Object.entries(rateLimitData.category_limits).forEach(
          ([category, limits]) => {
            rows.push({
              type: `${
                category.charAt(0).toUpperCase() + category.slice(1)
              } Events`,
              limit: `${limits.Limit}/sec`,
              burst: `${limits.Burst} events`,
              status: this.getStatusBadge(true),
            });
          }
        );
      }

      // Render the table
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

      console.log("✅ Policy limits loaded successfully");
    } catch (error) {
      console.error("Error loading policy limits:", error);
      const tbody = document.getElementById("policy-limits-table");
      if (tbody) {
        tbody.innerHTML = `
        <tr>
          <td colspan="4" class="px-6 py-4 text-center text-red-400">
            ⚠️ Failed to load policy configuration
          </td>
        </tr>
      `;
      }
    }
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
