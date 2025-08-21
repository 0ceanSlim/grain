(function () {
  // Encapsulate in IIFE to avoid conflicts
  console.log("Event page component script starting");

  // Event state
  let eventData = {
    eventId: "",
    event: null,
    jsonVisible: false,
  };

  // Initialize event page when component loads
  function initEventPage() {
    console.log("Event page component loaded");

    // Extract event ID from URL
    const eventId = window.location.pathname.replace("/e/", "");
    if (!eventId) {
      showError("No event ID provided");
      return;
    }

    eventData.eventId = eventId;
    setElementText("event-id", eventId);

    // Start the event loading process
    loadEvent();
  }

  async function loadEvent() {
    try {
      console.log("Loading event:", eventData.eventId);

      // Fetch event data using API
      const event = await fetchEvent(eventData.eventId);
      if (!event) {
        throw new Error("Event not found");
      }

      eventData.event = event;
      displayEvent(event);
    } catch (error) {
      console.error("Failed to load event:", error);
      showError(error.message);
    } finally {
      hideElement("loading");
      showElement("event-content");
    }
  }

  async function fetchEvent(eventId) {
    try {
      const response = await fetch(
        `/api/v1/events/query?ids=${encodeURIComponent(eventId)}`
      );

      if (!response.ok) {
        if (response.status === 404) {
          throw new Error("Event not found");
        }
        throw new Error(`API returned ${response.status}`);
      }

      const result = await response.json();
      console.log("Event query result:", result);

      // Extract the first event from the events array
      if (result.events && result.events.length > 0) {
        return result.events[0];
      } else {
        throw new Error("Event not found");
      }
    } catch (error) {
      console.error("Failed to fetch event:", error);
      throw error;
    }
  }

  function displayEvent(event) {
    console.log("Displaying event:", event);

    // Set basic event info
    setElementText("event-kind", event.kind);
    setElementText("event-author", event.pubkey);
    setElementText("event-author-hex", event.pubkey);

    // Format and display creation time
    const createdDate = new Date(event.created_at * 1000);
    setElementText("event-created", formatDateTime(createdDate));

    // Display raw JSON initially
    displayRawEventJson(event);

    // Display tags if present
    displayEventTags(event.tags || []);

    // Set complete JSON for toggle
    setElementText("complete-event-json", JSON.stringify(event, null, 2));

    // Render kind-specific content
    renderKindSpecificContent(event);
  }

  function renderKindSpecificContent(event) {
    const container = document.getElementById("kind-specific-content");
    if (!container) return;

    // For now, just show raw JSON for all kinds
    // TODO: Add kind-specific rendering in future iterations
    container.innerHTML = `
        <div class="p-4 bg-gray-900 rounded-lg">
          <h4 class="mb-2 text-sm font-medium text-gray-400">Raw Event JSON</h4>
          <pre class="text-sm text-gray-300 whitespace-pre-wrap break-all">${JSON.stringify(
            event,
            null,
            2
          )}</pre>
        </div>
      `;
  }

  function displayRawEventJson(event) {
    setElementText("raw-event-json", JSON.stringify(event, null, 2));
  }

  function displayEventTags(tags) {
    const container = document.getElementById("event-tags");
    const tagsContainer = document.getElementById("event-tags-container");

    if (!tags || tags.length === 0) {
      hideElement("event-tags-container");
      return;
    }

    showElement("event-tags-container");

    container.innerHTML = tags
      .map((tag, index) => {
        const tagType = tag[0] || "unknown";
        const tagValues = tag.slice(1).join(", ");

        return `
          <div class="flex items-start gap-2 p-2 bg-gray-700 rounded">
            <span class="px-2 py-1 text-xs font-mono text-white bg-gray-600 rounded">${tagType}</span>
            <span class="text-sm text-gray-300 break-all">${tagValues}</span>
          </div>
        `;
      })
      .join("");
  }

  function formatDateTime(date) {
    return date.toLocaleString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      timeZoneName: "short",
    });
  }

  // Action functions
  window.copyEventId = async function () {
    try {
      await navigator.clipboard.writeText(eventData.eventId);
      showToast("Event ID copied!");
    } catch (err) {
      console.error("Failed to copy event ID:", err);
      showToast("Failed to copy event ID", "error");
    }
  };

  window.copyAuthor = async function () {
    try {
      const authorPubkey =
        document.getElementById("event-author-hex").textContent;
      await navigator.clipboard.writeText(authorPubkey);
      showToast("Author pubkey copied!");
    } catch (err) {
      console.error("Failed to copy author:", err);
      showToast("Failed to copy author", "error");
    }
  };

  window.toggleEventJson = function () {
    const jsonContainer = document.getElementById("json-container");
    const toggleBtn = document.getElementById("toggle-json-btn");

    if (eventData.jsonVisible) {
      hideElement("json-container");
      toggleBtn.textContent = "Show Event JSON";
      eventData.jsonVisible = false;
    } else {
      showElement("json-container");
      toggleBtn.textContent = "Hide Event JSON";
      eventData.jsonVisible = true;
    }
  };

  window.viewAuthorProfile = function () {
    if (!eventData.event) return;

    try {
      // Navigate to author's profile page
      const authorPubkey = eventData.event.pubkey;
      const profileUrl = `/p/${authorPubkey}`;

      // Use HTMX to load profile page
      htmx.ajax("GET", "/views/components/profile-page.html", "#main-content");
      window.history.pushState({}, "", profileUrl);

      console.log("Navigated to author profile:", profileUrl);
    } catch (error) {
      console.error("Failed to navigate to author profile:", error);
      showToast("Failed to navigate to profile", "error");
    }
  };

  window.retryLoadEvent = function () {
    hideElement("error");
    showElement("loading");
    hideElement("event-content");
    loadEvent();
  };

  // Utility functions
  function setElementText(id, text) {
    const element = document.getElementById(id);
    if (element) {
      element.textContent = text;
    }
  }

  function showElement(id) {
    const element = document.getElementById(id);
    if (element) {
      element.classList.remove("hidden");
    }
  }

  function hideElement(id) {
    const element = document.getElementById(id);
    if (element) {
      element.classList.add("hidden");
    }
  }

  function showError(message) {
    setElementText("error-message", message);
    hideElement("loading");
    showElement("error");
    hideElement("event-content");
  }

  function showToast(message, type = "success") {
    // Simple toast implementation
    const toast = document.createElement("div");
    const bgColor = type === "error" ? "bg-red-600" : "bg-green-600";

    toast.className = `fixed top-4 right-4 ${bgColor} text-white px-4 py-2 rounded-lg shadow-lg z-50 transition-opacity`;
    toast.textContent = message;

    document.body.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = "0";
      setTimeout(() => {
        document.body.removeChild(toast);
      }, 300);
    }, 3000);
  }

  // Initialize when the page loads
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initEventPage);
  } else {
    initEventPage();
  }
})();
