const CACHE_NAME = "grain-v4"; // Updated version to force cache refresh
const STATIC_CACHE_URLS = [
  "/",
  "/static/js/navigation.js",
  "/static/js/routing.js",
  "/static/js/settings.js",
  "/static/js/dropdown.js",
  // Note: Auth scripts are deliberately excluded from caching
];

// Install event - cache only truly static resources
self.addEventListener("install", (event) => {
  console.log("[SW] Installing service worker v4 - Auth-aware");
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) => {
        console.log("[SW] Caching static resources only");
        return cache.addAll(STATIC_CACHE_URLS);
      })
      .then(() => {
        console.log("[SW] Skip waiting");
        return self.skipWaiting();
      })
      .catch((err) => {
        console.error("[SW] Install failed:", err);
      })
  );
});

// Activate event - clean up old caches
self.addEventListener("activate", (event) => {
  console.log("[SW] Activating service worker v3");
  event.waitUntil(
    caches
      .keys()
      .then((cacheNames) => {
        return Promise.all(
          cacheNames.map((cacheName) => {
            if (cacheName !== CACHE_NAME) {
              console.log("[SW] Deleting old cache:", cacheName);
              return caches.delete(cacheName);
            }
          })
        );
      })
      .then(() => {
        console.log("[SW] Claiming clients");
        return self.clients.claim();
      })
  );
});

// Fetch event - HTMX-aware caching strategy
self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET requests
  if (request.method !== "GET") {
    return;
  }

  // Skip WebSocket upgrade requests
  if (request.headers.get("upgrade") === "websocket") {
    return;
  }

  // CRITICAL: Skip NIP-11 requests - always go to network for JSON
  if (
    request.headers.get("accept") &&
    request.headers.get("accept").includes("application/nostr+json")
  ) {
    console.log(
      "[SW] Skipping NIP-11 request (need fresh JSON):",
      url.pathname
    );
    return;
  }

  // CRITICAL: Skip HTMX requests - let them always go to network
  if (request.headers.get("hx-request") === "true") {
    console.log(
      "[SW] Skipping HTMX request (letting through to network):",
      url.pathname
    );
    return;
  }

  // Skip API calls - always go to network
  if (url.pathname.startsWith("/api/")) {
    console.log("[SW] Skipping API request:", url.pathname);
    return;
  }

  // Skip view templates - always go to network for fresh content
  if (url.pathname.startsWith("/views/")) {
    console.log(
      "[SW] Skipping view template (fresh content needed):",
      url.pathname
    );
    return;
  }

  // CRITICAL: Skip auth scripts - never cache these
  if (url.pathname.startsWith("/static/js/auth/")) {
    console.log("[SW] Skipping auth script (always fresh):", url.pathname);
    return;
  }

  // Skip auth-related paths
  if (url.pathname.startsWith("/login") || url.pathname.startsWith("/logout")) {
    return;
  }

  event.respondWith(
    caches.match(request).then((cachedResponse) => {
      if (cachedResponse) {
        console.log("[SW] Serving from cache:", url.pathname);
        return cachedResponse;
      }

      console.log("[SW] Fetching from network:", url.pathname);
      return fetch(request)
        .then((response) => {
          // Don't cache non-successful responses
          if (
            !response ||
            response.status !== 200 ||
            response.type !== "basic"
          ) {
            return response;
          }

          // Only cache truly static assets
          if (shouldCache(url)) {
            const responseToCache = response.clone();
            caches.open(CACHE_NAME).then((cache) => {
              console.log("[SW] Caching new static resource:", url.pathname);
              cache.put(request, responseToCache);
            });
          }

          return response;
        })
        .catch((err) => {
          console.error("[SW] Fetch failed:", err);

          // For navigation requests, serve cached root if available
          if (request.mode === "navigate") {
            return caches.match("/").then((cachedRoot) => {
              if (cachedRoot) {
                console.log("[SW] Serving cached root for offline navigation");
                return cachedRoot;
              }
              throw err;
            });
          }

          throw err;
        });
    })
  );
});

// Helper function to determine if a resource should be cached
function shouldCache(url) {
  // NEVER cache auth scripts
  if (url.pathname.startsWith("/static/js/auth/")) {
    return false;
  }

  // Cache other static JavaScript files (but be selective)
  if (url.pathname.startsWith("/static/js/")) {
    // Only cache core navigation/routing scripts
    const allowedScripts = [
      "/static/js/navigation.js",
      "/static/js/routing.js",
      "/static/js/profile.js",
      "/static/js/settings.js",
      "/static/js/dropdown.js",
    ];
    return allowedScripts.includes(url.pathname);
  }

  // Cache static CSS files
  if (
    url.pathname.startsWith("/static/css/") ||
    url.pathname.startsWith("/style/")
  ) {
    return true;
  }

  // Cache static icons
  if (url.pathname.startsWith("/static/icons/")) {
    return true;
  }

  // Cache manifest and service worker
  if (url.pathname === "/manifest.json" || url.pathname === "/sw.js") {
    return true;
  }

  // Cache the root page only (not view partials)
  if (url.pathname === "/") {
    return true;
  }

  return false;
}

// Listen for messages from the client
self.addEventListener("message", (event) => {
  if (event.data && event.data.type === "SKIP_WAITING") {
    console.log("[SW] Received skip waiting message");
    self.skipWaiting();
  }
});

// Background sync for when connection is restored
self.addEventListener("sync", (event) => {
  if (event.tag === "nostr-sync") {
    console.log("[SW] Background sync: nostr-sync");
    event.waitUntil(syncNostrData());
  }
});

// Sync function for Nostr data
async function syncNostrData() {
  try {
    console.log("[SW] Syncing Nostr data...");
    // Implementation would depend on your specific Nostr sync needs
  } catch (error) {
    console.error("[SW] Sync failed:", error);
  }
}
