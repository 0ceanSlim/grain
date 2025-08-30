/**
 * Fixed client-side routing for HTMX navigation
 * Handles initial page loads and browser back/forward buttons
 * Updated to work when loaded dynamically after DOM is ready
 */

console.log("[ROUTING] Script loaded, DOM ready state:", document.readyState);

// Function to handle route loading
function handleRouteLoad() {
  const currentPath = window.location.pathname;
  let targetView = "/views/home.html"; // default

  console.log("[ROUTING] Handling route load for path:", currentPath);

  // Map URL paths to view files
  switch (currentPath) {
    case "/":
      targetView = "/views/home.html";
      break;
    case "/settings":
      // Check if user has session before loading settings
      fetch("/api/v1/session")
        .then((response) => {
          if (response.ok) {
            targetView = "/views/settings.html";
          } else {
            // No session, redirect to home and show login modal
            targetView = "/views/home.html";
            window.history.pushState({}, "", "/");

            // Show login modal after home loads
            setTimeout(() => {
              const loginModal = document.getElementById("login-modal");
              if (loginModal) {
                loginModal.classList.remove("hidden");
              }
            }, 100);
          }
          console.log("[ROUTING] Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        })
        .catch(() => {
          // Error checking session, go to home and show login modal
          targetView = "/views/home.html";
          window.history.pushState({}, "", "/");
          console.log("[ROUTING] Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        });
      return; // Exit early since we're handling this asynchronously
    default:
      // Check if this is a profile route (/p/<identifier>)
      if (currentPath.startsWith("/p/")) {
        const identifier = currentPath.replace("/p/", "");
        if (identifier) {
          console.log("[ROUTING] Loading profile for identifier:", identifier);
          // Load profile component directly
          htmx.ajax("GET", "/views/components/profile-page.html", {
            target: "#main-content",
          });
          return; // Exit early
        }
      }

      // Check if this is an event route (/e/<eventID>)
      if (currentPath.startsWith("/e/")) {
        const eventId = currentPath.replace("/e/", "");
        if (eventId) {
          console.log("[ROUTING] Loading event for ID:", eventId);
          // Load event component directly
          htmx.ajax("GET", "/views/components/event-page.html", {
            target: "#main-content",
          });
          return; // Exit early
        }
      }

      // Default to home for unrecognized routes
      targetView = "/views/home.html";
      window.history.replaceState({}, "", "/");
      break;
  }

  console.log("[ROUTING] Loading view:", targetView);
  htmx.ajax("GET", targetView, { target: "#main-content" });
}

// Handle browser back/forward buttons
window.addEventListener("popstate", function (event) {
  const currentPath = window.location.pathname;
  let targetView = "/views/home.html";

  console.log("[ROUTING] Popstate - current path:", currentPath);

  switch (currentPath) {
    case "/":
      targetView = "/views/home.html";
      break;
    case "/settings":
      // Check if user has valid session for settings route
      fetch("/api/v1/session")
        .then((response) => {
          if (response.ok) {
            targetView = "/views/settings.html";
          } else {
            // No session, redirect to home
            targetView = "/views/home.html";
            window.history.replaceState({}, "", "/");
          }
          console.log("[ROUTING] Loading view via popstate:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        })
        .catch(() => {
          // Error checking session, go to home
          targetView = "/views/home.html";
          window.history.replaceState({}, "", "/");
          console.log("[ROUTING] Loading view via popstate:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        });
      return; // Exit early since we're handling this asynchronously
    default:
      // Check if this is a profile route (/p/<identifier>)
      if (currentPath.startsWith("/p/")) {
        const identifier = currentPath.replace("/p/", "");
        if (identifier) {
          console.log(
            "[ROUTING] Popstate - Loading profile for identifier:",
            identifier
          );
          // Load profile component directly
          htmx.ajax("GET", "/views/components/profile-page.html", {
            target: "#main-content",
          });
          return; // Exit early
        }
      }

      // Default to home for unrecognized routes
      targetView = "/views/home.html";
      window.history.replaceState({}, "", "/");
      break;
  }

  console.log("[ROUTING] Loading view via popstate:", targetView);
  htmx.ajax("GET", targetView, { target: "#main-content" });
});

console.log("[ROUTING] Event listeners registered");

// Safe initialization when DOM is ready
function initializeRouting() {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", handleRouteLoad);
  } else {
    handleRouteLoad();
  }
}

// Initialize routing
initializeRouting();
