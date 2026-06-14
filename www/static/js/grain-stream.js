/**
 * Live update channel (SSE).
 *
 * Opens a single EventSource to /api/v1/stream when a session exists and
 * re-dispatches each message as a `grain:stream` CustomEvent on window, so any
 * page (relay manager, profile, media servers) can react to live updates
 * without managing its own connection. Loaded app-wide so the connection
 * survives SPA navigation. (#87 live-sync, #77 streaming fetch.)
 */
(function () {
  let es = null;

  function connect() {
    if (es) return;
    try {
      es = new EventSource("/api/v1/stream");
      es.onmessage = function (e) {
        let msg;
        try {
          msg = JSON.parse(e.data);
        } catch (err) {
          return;
        }
        window.dispatchEvent(new CustomEvent("grain:stream", { detail: msg }));
      };
      es.onerror = function () {
        // EventSource auto-reconnects; nothing to do.
      };
    } catch (e) {
      /* EventSource unsupported — degrade silently */
    }
  }

  function disconnect() {
    if (es) {
      try {
        es.close();
      } catch (e) {}
      es = null;
    }
  }

  window.grainStream = { connect: connect, disconnect: disconnect };

  // Connect only when logged in, so logged-out pages don't spin retrying a 401.
  fetch("/api/v1/session")
    .then(function (r) {
      if (r.ok) connect();
    })
    .catch(function () {});
})();
