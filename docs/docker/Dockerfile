FROM alpine:latest

# Install dependencies for downloading and extracting releases
RUN apk add --no-cache ca-certificates curl jq unzip tar gzip wget

WORKDIR /tmp

# Detect architecture and download appropriate release
RUN ARCH=$(uname -m) && \
    case $ARCH in \
        x86_64) GRAIN_ARCH="amd64" ;; \
        aarch64) GRAIN_ARCH="arm64" ;; \
        *) echo "Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "Detected architecture: $ARCH -> grain-linux-$GRAIN_ARCH" && \
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/0ceanslim/grain/releases/latest | jq -r .tag_name) && \
    echo "Downloading GRAIN version: $LATEST_RELEASE for linux-$GRAIN_ARCH" && \
    DOWNLOAD_URL="https://github.com/0ceanslim/grain/releases/download/$LATEST_RELEASE/grain-linux-$GRAIN_ARCH.tar.gz" && \
    curl -L "$DOWNLOAD_URL" -o grain-release.tar.gz && \
    echo "Extracting release..." && \
    tar -xzf grain-release.tar.gz && \
    EXTRACTED_DIR=$(tar -tzf grain-release.tar.gz | head -1 | cut -f1 -d"/") && \
    echo "Moving files from $EXTRACTED_DIR to /app..." && \
    mkdir -p /app && \
    mv "$EXTRACTED_DIR"/* /app/ && \
    chmod +x /app/grain && \
    rm -rf /tmp/*

WORKDIR /app

# Create non-root user for security
RUN addgroup -g 1001 grain && \
    adduser -D -s /bin/sh -u 1001 -G grain grain && \
    chown -R grain:grain /app

USER grain

EXPOSE 8181

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8181/ || exit 1

CMD ["./grain"]