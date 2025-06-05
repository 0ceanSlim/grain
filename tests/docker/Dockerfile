FROM golang:1.23-alpine

# Add label to identify test images
LABEL test-image=grain

# Install basic tools for testing
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 go build -o grain .

# Copy example configs and modify for Docker
RUN cp www/static/examples/config.example.yml config.yml && \
    cp www/static/examples/whitelist.example.yml whitelist.yml && \
    cp www/static/examples/blacklist.example.yml blacklist.yml && \
    cp www/static/examples/relay_metadata.example.json relay_metadata.json && \
    sed -i 's/localhost:27017/mongo:27017/g' config.yml && \
    sed -i 's/:8181/:8182/g' config.yml

EXPOSE 8182

CMD ["./grain"]
