FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates curl jq

WORKDIR /build

# Get latest release tag and download source
RUN LATEST_RELEASE=$(curl -s https://api.github.com/repos/0ceanslim/grain/releases/latest | jq -r .tag_name) && \
    echo "Building GRAIN version: $LATEST_RELEASE" && \
    curl -L "https://github.com/0ceanslim/grain/archive/$LATEST_RELEASE.tar.gz" -o source.tar.gz && \
    tar -xzf source.tar.gz --strip-components=1

# Download go dependencies and build
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o grain .

# Production stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy binary and www assets from builder
COPY --from=builder /build/grain ./
COPY --from=builder /build/www ./www

# Create non-root user for security
RUN addgroup -g 1001 grain && \
    adduser -D -s /bin/sh -u 1001 -G grain grain && \
    chown -R grain:grain /app

USER grain

EXPOSE 8181

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8181/ || exit 1

CMD ["./grain"]