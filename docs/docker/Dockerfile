FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates curl jq unzip

WORKDIR /build

# Get latest release tag and download source
RUN LATEST_RELEASE=$(curl -s https://api.github.com/repos/0ceanslim/grain/releases/latest | jq -r .tag_name) && \
    echo "Building GRAIN version: $LATEST_RELEASE" && \
    curl -L "https://github.com/0ceanslim/grain/archive/$LATEST_RELEASE.tar.gz" -o source.tar.gz && \
    tar -xzf source.tar.gz --strip-components=1

# Download go dependencies and build
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -o grain .

# Download www assets from release
RUN LATEST_RELEASE=$(curl -s https://api.github.com/repos/0ceanslim/grain/releases/latest | jq -r .tag_name) && \
    curl -L "https://github.com/0ceanslim/grain/releases/download/$LATEST_RELEASE/www.zip" -o www.zip && \
    unzip www.zip

# Production stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary and www assets from builder
COPY --from=builder /build/grain ./
COPY --from=builder /build/www ./www

# Setup default configs for Docker
RUN sed -i 's/localhost:27017/mongo:27017/g' config.yml

EXPOSE 8181

CMD ["./grain"]