// Per-user client-tag preference (#99), persisted client-side and sent with each
// build request to override the server default. Absent = on (the default).
(function () {
  "use strict";
  var KEY = "grain.clientTag"; // "on" | "off"
  window.grainClientTag = {
    // enabled() is the value to send with build requests. Default on.
    enabled: function () {
      return localStorage.getItem(KEY) !== "off";
    },
    set: function (on) {
      localStorage.setItem(KEY, on ? "on" : "off");
    },
  };
})();
