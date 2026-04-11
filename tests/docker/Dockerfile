FROM golang:1.23-alpine

# Add label to identify test images
LABEL test-image=grain

# Install tools for testing and C toolchain for nostrdb
RUN apk add --no-cache git ca-certificates gcc musl-dev make autoconf automake libtool bash

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Initialize submodules and build nostrdb C library
RUN git init 2>/dev/null || true && \
    git submodule update --init --recursive 2>/dev/null || true && \
    cd server/db/nostrdb && bash build.sh

# Build the binary with CGO enabled for nostrdb
RUN CGO_ENABLED=1 go build -buildvcs=false -o grain .

# Modify config for test environment
RUN sed -i 's/:8181/:8182/g' config.yml

EXPOSE 8182

CMD ["./grain"]
