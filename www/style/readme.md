# GRAIN Styling

This directory contains the TailwindCSS v4 configuration and source files for GRAIN's web interface styling. The repository no longer includes a pre-built minified CSS file - you must build it using the TailwindCSS CLI.

## Building CSS

To rebuild the CSS after making changes to `input.css` or your templates, use the [TailwindCSS standalone CLI](https://github.com/tailwindlabs/tailwindcss/releases):

```bash
cd www/style
tailwindcss -i input.css -o tailwind.min.css --minify
```

## Development Workflow

For active development, run the watcher to automatically rebuild CSS when files change:

```bash
cd www/style
tailwindcss -i input.css -o tailwind.min.css --watch --minify
```

## TailwindCSS v4 Configuration

This project uses **TailwindCSS v4** with CSS-first configuration:

- **Source files**: Explicitly defined in `input.css` using `@source` directives
- **Theme customization**: Done via `@theme` directive in CSS instead of JavaScript config
- **Auto-detection**: Scans `../views/**/*.html` and `../static/**/*.js` for Tailwind classes
- **No config file**: JavaScript configuration has been removed in favor of CSS-first approach

## Theme System

GRAIN uses a custom dark/light theme system:

- **Default theme**: Dark mode (because all things should be dark)
- **Theme switching**: CSS custom properties with `data-theme="light"` attribute
- **Custom colors**: Defined in `@theme` directive with semantic naming (bgPrimary, textPrimary, etc.)

## File Structure

```
www/style/
├── input.css          # Source CSS with TailwindCSS v4 imports and custom theme
├── tailwind.min.css   # Generated minified CSS (build artifact)
└── readme.md          # This file
```

## Production Builds

The Docker build process automatically:

1. Downloads the TailwindCSS CLI
2. Builds the minified CSS
3. Bundles it with the application

No manual CSS building is required for production releases.
