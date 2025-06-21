/**
 * Simple client-side routing for HTMX navigation
 * Handles initial page loads and browser back/forward buttons
 * Updated to use login modal instead of login route
 */

// Handle initial route loading based on current URL path
document.addEventListener("DOMContentLoaded", function () {
  const currentPath = window.location.pathname;
  let targetView = "/views/home.html"; // default

  console.log("Current path:", currentPath);

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
          console.log("Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        })
        .catch(() => {
          // Error checking session, go to home and show login modal
          targetView = "/views/home.html";
          window.history.pushState({}, "", "/");
          console.log("Loading view:", targetView);
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

  console.log("Loading initial view:", targetView, "for path:", currentPath);

  // Load the appropriate view
  htmx.ajax("GET", targetView, { target: "#main-content" });
});

// Handle browser back/forward buttons
window.addEventListener("popstate", function (event) {
  const currentPath = window.location.pathname;
  let targetView = "/views/home.html";

  console.log("Popstate - current path:", currentPath);

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
          console.log("Loading view via popstate:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        })
        .catch(() => {
          // Error checking session, go to home
          targetView = "/views/home.html";
          window.history.replaceState({}, "", "/");
          console.log("Loading view via popstate:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        });
      return; // Exit early since we're handling this asynchronously
    default:
      targetView = "/views/home.html";
      window.history.replaceState({}, "", "/");
      break;
  }

  console.log("Loading view via popstate:", targetView);
  htmx.ajax("GET", targetView, { target: "#main-content" });
});
