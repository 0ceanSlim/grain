// GRAIN theme swapper.
//
// Mirrors the pubkey-quest pattern (src/systems/themeManager.js):
//   - data-theme attribute on <html>
//   - localStorage persistence
//   - 'grain:theme-changed' DOM event so any listener can react
//
// Loaded synchronously near the top of layout.html so the initial
// theme is applied BEFORE first paint and there's no light-flash on
// dark-preferring browsers. The dropdown UI is initialized later via
// initThemeSwapper(selectEl) when the header has rendered.
//
// Adding a new theme: append to THEMES below AND add a
// :root[data-theme="<id>"] block in www/style/input.css.

(function () {
  "use strict";

  const STORAGE_KEY = "grain-theme";
  const DEFAULT = "dark";

  // Keep this list in sync with the data-theme blocks in input.css.
  const THEMES = [
    { id: "dark", label: "Dark" },
    { id: "light", label: "Light" },
  ];

  function getTheme() {
    try {
      const v = localStorage.getItem(STORAGE_KEY);
      if (v && THEMES.some((t) => t.id === v)) return v;
    } catch (_) {
      // localStorage unavailable (private mode, embedded contexts).
      // Falling through to default is the right move; the swapper
      // will still flip themes for the current session.
    }
    return DEFAULT;
  }

  function setTheme(id) {
    if (!THEMES.some((t) => t.id === id)) return;
    document.documentElement.setAttribute("data-theme", id);
    try {
      localStorage.setItem(STORAGE_KEY, id);
    } catch (_) {}
    document.dispatchEvent(
      new CustomEvent("grain:theme-changed", { detail: { theme: id } })
    );
  }

  // Apply before paint. Subsequent calls from initThemeSwapper just
  // re-confirm the current value, which is harmless.
  setTheme(getTheme());

  function initThemeSwapper(selectEl) {
    if (!selectEl) return;
    // Populate options. Done in JS rather than the template so adding
    // a theme is a one-line change to THEMES — no template editing.
    selectEl.innerHTML = "";
    for (const t of THEMES) {
      const opt = document.createElement("option");
      opt.value = t.id;
      opt.textContent = t.label;
      selectEl.appendChild(opt);
    }
    selectEl.value = getTheme();
    selectEl.addEventListener("change", (e) => setTheme(e.target.value));
  }

  // Expose the small public surface for inline scripts and the header
  // template. Kept minimal — anything more would be the start of a
  // module system we don't currently have.
  window.GrainTheme = {
    THEMES,
    get: getTheme,
    set: setTheme,
    initSwapper: initThemeSwapper,
  };
})();
