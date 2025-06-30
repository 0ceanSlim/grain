/**
 * Bunker handling
 */
function connectBunker() {
  const bunkerUrl = document.getElementById("bunker-url").value.trim();

  if (!bunkerUrl) {
    showAuthResult("error", "Please enter a bunker URL");
    return;
  }

  if (!bunkerUrl.startsWith("bunker://")) {
    showAuthResult("error", "Invalid bunker URL format");
    return;
  }

  showAuthResult("loading", "Connecting to bunker...");

  // TODO: Implement NIP-46 bunker connection logic
  setTimeout(() => {
    showAuthResult("error", "Bunker integration coming soon!");
  }, 1000);
}
