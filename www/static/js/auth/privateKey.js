/**
 * Private key handling
 */
function connectPrivateKey() {
  const privateKey = document.getElementById("private-key").value.trim();
  const sessionPassword = document.getElementById("session-password").value;

  if (!privateKey || !sessionPassword) {
    showAuthResult("error", "Please fill in all fields");
    return;
  }

  showAuthResult("loading", "Encrypting and storing key...");

  // TODO: Implement private key encryption and storage
  setTimeout(() => {
    showAuthResult("error", "Private key authentication coming soon!");
  }, 1000);
}
