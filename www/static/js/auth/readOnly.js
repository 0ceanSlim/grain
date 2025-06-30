/**
 * Read-only login handling - FIXED API endpoint
 */
function connectReadOnly() {
  const pubkey = document.getElementById("readonly-pubkey").value.trim();

  if (!pubkey) {
    showAuthResult("error", "Please enter a public key");
    return;
  }

  if (!isValidPublicKey(pubkey)) {
    showAuthResult("error", "Invalid public key format");
    return;
  }

  showAuthResult("loading", "Creating read-only session...");

  // Keep npub as-is, let backend handle conversion
  const sessionRequest = {
    public_key: pubkey,
    requested_mode: "read_only",
    signing_method: "none",
  };

  // FIXED: Use correct API endpoint
  fetch("/api/v1/auth/login", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(sessionRequest),
  })
    .then((response) => response.json())
    .then((result) => {
      if (result.success) {
        showAuthResult("success", "Read-only session created!");
        setTimeout(() => {
          hideAuthModal();
          if (window.updateNavigation) {
            window.updateNavigation();
          }
          if (result.redirect_url) {
            setTimeout(() => {
              htmx.ajax("GET", "/views/profile.html", "#main-content");
              window.history.pushState({}, "", "/profile");
            }, 500);
          }
        }, 1000);
      } else {
        showAuthResult("error", result.message || "Login failed");
      }
    })
    .catch((error) => {
      console.error("Read-only login error:", error);
      showAuthResult("error", "Connection failed");
    });
}
