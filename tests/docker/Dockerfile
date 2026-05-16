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

# Build the binary with CGO enabled for nostrdb
RUN CGO_ENABLED=1 go build -buildvcs=false -o grain .

# Each service bind-mounts its config into /app/testdata/config.yml
# (plus sibling blacklist.yml / whitelist.yml where applicable).
RUN mkdir -p /app/testdata

ENTRYPOINT ["./grain", "--data-dir", "/app/testdata"]
