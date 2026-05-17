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
// initThemeSwapper(buttonEl, panelEl) once the header has rendered.
//
// Adding a new theme: append to THEMES below AND add a
// :root[data-theme="<id>"] block in www/style/input.css. Each entry
// also carries a `swatch` color so the dropdown shows a tiny tile
// next to the label — easy visual scan when picking.

(function () {
  "use strict";

  const STORAGE_KEY = "grain-theme";
  const DEFAULT = "dark";

  // Keep this list in sync with the data-theme blocks in input.css.
  // `swatch` is a representative surface-base color used in the
  // dropdown row preview.
  const THEMES = [
    { id: "dark", label: "Dark", swatch: "#09080f" },
    { id: "light", label: "Light", swatch: "#f4f1fa" },
    { id: "midnight", label: "Midnight", swatch: "#0c0a1e" },
    { id: "matrix", label: "Matrix", swatch: "#000000" },
    { id: "grain", label: "Grain", swatch: "#0d0f0c" },
    { id: "solar", label: "Solar", swatch: "#002b36" },
    { id: "candy", label: "Candy", swatch: "#fff5f9" },
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

  // initThemeSwapper(buttonEl, panelEl)
  //
  // buttonEl: the trigger (🎨 icon button in the header)
  // panelEl: the dropdown container — populated with one row per
  //          theme by this function. Toggled hidden/visible by
  //          clicking the button, dismissed by clicking outside.
  function initThemeSwapper(buttonEl, panelEl) {
    if (!buttonEl || !panelEl) return;

    // Render rows. Inline styles read from the design tokens so each
    // row's hover/active state retints when themes change without us
    // having to re-render.
    panelEl.innerHTML = "";
    const current = getTheme();
    for (const t of THEMES) {
      const row = document.createElement("button");
      row.type = "button";
      row.dataset.theme = t.id;
      row.className =
        "flex items-center w-full gap-2 px-3 py-2 text-sm text-left hover:bg-surface-hover" +
        (t.id === current ? " bg-surface-hover" : "");
      row.innerHTML =
        '<span class="inline-block w-3 h-3 rounded-sm border border-border" style="background:' +
        t.swatch +
        '"></span><span>' +
        t.label +
        "</span>";
      row.addEventListener("click", function () {
        setTheme(t.id);
        // Mark this row as the current selection and clear others
        // so the highlight follows without a re-render.
        Array.from(panelEl.children).forEach(function (child) {
          child.classList.remove("bg-surface-hover");
        });
        row.classList.add("bg-surface-hover");
        closePanel();
      });
      panelEl.appendChild(row);
    }

    function openPanel() {
      panelEl.classList.remove("hidden");
      setTimeout(function () {
        document.addEventListener("click", clickOutside);
      }, 0);
    }
    function closePanel() {
      panelEl.classList.add("hidden");
      document.removeEventListener("click", clickOutside);
    }
    function clickOutside(e) {
      if (!panelEl.contains(e.target) && !buttonEl.contains(e.target)) {
        closePanel();
      }
    }

    buttonEl.addEventListener("click", function (e) {
      e.stopPropagation();
      if (panelEl.classList.contains("hidden")) openPanel();
      else closePanel();
    });
  }

  window.GrainTheme = {
    THEMES,
    get: getTheme,
    set: setTheme,
    initSwapper: initThemeSwapper,
  };
})();
