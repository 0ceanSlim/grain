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
    case "/profile":
      // Check if user has session before loading profile
      fetch("/api/v1/session")
        .then((response) => {
          if (response.ok) {
            targetView = "/views/profile.html";
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
          console.log("[ROUTING] Error checking session, loading:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });

          // Show login modal after home loads
          setTimeout(() => {
            const loginModal = document.getElementById("login-modal");
            if (loginModal) {
              loginModal.classList.remove("hidden");
            }
          }, 100);
        });
      return; // Exit early since we're handling this asynchronously
    default:
      // For any other path, default to home
      targetView = "/views/home.html";
      window.history.pushState({}, "", "/");
      break;
  }

  console.log(
    "[ROUTING] Loading initial view:",
    targetView,
    "for path:",
    currentPath
  );

  // Check if HTMX is available before trying to load
  if (typeof htmx !== "undefined") {
    htmx.ajax("GET", targetView, { target: "#main-content" });
  } else {
    console.error("[ROUTING] HTMX not available, cannot load view");
    setTimeout(handleRouteLoad, 100); // Retry in 100ms
  }
}

// Execute route loading immediately if DOM is already ready
// or wait for DOMContentLoaded if it hasn't fired yet
if (document.readyState === "loading") {
  console.log("[ROUTING] DOM still loading, waiting for DOMContentLoaded");
  document.addEventListener("DOMContentLoaded", handleRouteLoad);
} else {
  console.log("[ROUTING] DOM already ready, executing route load immediately");
  // Small delay to ensure HTMX is ready
  setTimeout(handleRouteLoad, 50);
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
    case "/profile":
      // Check if user has valid session for profile route
      fetch("/api/v1/session")
        .then((response) => {
          if (response.ok) {
            targetView = "/views/profile.html";
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
      targetView = "/views/home.html";
      window.history.replaceState({}, "", "/");
      break;
  }

  console.log("[ROUTING] Loading view via popstate:", targetView);
  htmx.ajax("GET", targetView, { target: "#main-content" });
});

console.log("[ROUTING] Event listeners registered");
