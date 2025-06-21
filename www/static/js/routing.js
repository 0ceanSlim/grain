/**
 * Simple client-side routing for HTMX navigation
 * Handles initial page loads and browser back/forward buttons
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
            // No session, redirect to login
            targetView = "/views/login.html";
            window.history.pushState({}, "", "/login");
          }
          console.log("Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        })
        .catch(() => {
          // Error checking session, go to login
          targetView = "/views/login.html";
          window.history.pushState({}, "", "/login");
          console.log("Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        });
      return; // Exit early since we're handling this asynchronously
    case "/login":
      // Check if user is already logged in before loading login page
      fetch("/api/v1/session")
        .then((response) => {
          if (response.ok) {
            // User is already logged in, redirect to profile
            console.log("User already logged in, redirecting to profile");
            targetView = "/views/profile.html";
            window.history.pushState({}, "", "/profile");
          } else {
            // Not logged in, show login page
            targetView = "/views/login.html";
          }
          console.log("Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
        })
        .catch(() => {
          // Error checking session, show login page
          targetView = "/views/login.html";
          console.log("Loading view:", targetView);
          htmx.ajax("GET", targetView, { target: "#main-content" });
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
      targetView = "/views/profile.html";
      break;
    case "/login":
      targetView = "/views/login.html";
      break;
    default:
      targetView = "/views/home.html";
      break;
  }

  console.log("Loading view via popstate:", targetView);
  htmx.ajax("GET", targetView, { target: "#main-content" });
});
