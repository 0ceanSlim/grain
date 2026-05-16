FROM golang:1.23-alpine

# Add label to identify test images
LABEL test-image=grain

# Install tools for testing and C toolchain for nostrdb
RUN apk add --no-cache git ca-certificates gcc musl-dev make autoconf automake libtool bash dos2unix

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Fix CRLF line endings in C source files (Windows git checkout)
RUN find server/db/nostrdb -type f \! -name '*.o' \! -name '*.a' \! -name '*.png' \! -name '*.pdf' \
        -exec sed -i 's/\r$//' {} +

# Build nostrdb C library
RUN cd server/db/nostrdb && bash build.sh

# Generate the OpenAPI spec from swag annotations. main.go embeds
# docs/openapi/swagger.json via //go:embed, so a missing file fails
# the build with a clear "no matching files found" error. This step
# is the build-time half of the contract; the runtime half lives in
# server/api/docs.
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.4 \
    && swag init --parseDependency --parseInternal -g main.go -o docs/openapi -ot json

# Bundle swagger-ui-dist for the API docs page. The version is pinned
# rather than tracking "latest" so a surprise UI overhaul upstream
# can't change the look without a deliberate bump. Files land in
# www/static/swagger/ which is embedded via the existing //go:embed
# www/* directive in main.go.
ARG SWAGGER_UI_VERSION=5.17.14
RUN apk add --no-cache curl tar libstdc++ libgcc \
    && mkdir -p www/static/swagger \
    && curl -fsSL -o /tmp/swagger.tgz \
        "https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-${SWAGGER_UI_VERSION}.tgz" \
    && tar -xzf /tmp/swagger.tgz -C /tmp \
    && cp /tmp/package/swagger-ui-bundle.js \
          /tmp/package/swagger-ui-standalone-preset.js \
          /tmp/package/swagger-ui.css \
          www/static/swagger/ \
    && rm -rf /tmp/swagger.tgz /tmp/package

# Compile Tailwind. The Tailwind v3 CDN script in layout.html only
# processes inline utility classes — it never reads input.css, which
# means the @theme tokens (--color-surface-*, --color-accent, …)
# never reach the browser. Compiling input.css → tailwind.min.css
# and rewriting the layout to load it mirrors what
# .github/workflows/release.yml does for production builds, so test
# and prod render the same design tokens.
RUN curl -fsSL -o /tmp/tailwindcss \
        https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64-musl \
    && chmod +x /tmp/tailwindcss \
    && (cd www/style && /tmp/tailwindcss -i input.css -o tailwind.min.css --minify) \
    && sed -i 's|<script src="https://cdn.tailwindcss.com"></script>|<link rel="stylesheet" href="/style/tailwind.min.css">|g' www/views/templates/layout.html \
    && rm /tmp/tailwindcss

# Build the binary with CGO enabled for nostrdb
RUN CGO_ENABLED=1 go build -buildvcs=false -o grain .

# Each service bind-mounts its config into /app/testdata/config.yml
# (plus sibling blacklist.yml / whitelist.yml where applicable).
RUN mkdir -p /app/testdata

ENTRYPOINT ["./grain", "--data-dir", "/app/testdata"]
