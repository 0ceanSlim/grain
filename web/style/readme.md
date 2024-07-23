# Information

This repository INCLUDES a minified version of the custom css used to style the web views with themes defined in the input.css. If you want to change anything about the configuration or the input, you will need to rebuild the custom minified css by using the [Tailwind standalone CLI Tool](https://github.com/tailwindlabs/tailwindcss/releases).

For Tailwind to Rebuild the CSS, Tailwind must be run to compile the new styling.

To do this run:

```bash
tailwindcss -i web/style/input.css -o web/static/custom.min.css --minify
```

## Development

You can run a watcher while in development to automatically rebuild the `tailwind.min.css` whenever a file in the project directory is modified.

To do this run:

```bash
tailwindcss -i web/style/input.css -o web/static/custom.min.css --watch --minify
```

### Dark Mode

Yes... This framework is designed with "Dark Mode" as the default theme. As all things should be.
