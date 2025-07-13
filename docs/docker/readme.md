# GRAIN Docker Setup

Run GRAIN relay with Docker using either the latest stable release or pre-release binaries.

## Table of Contents

1. [Quick Start](#quick-start)

- [Option 1: Stable Release (Recommended)](#option-1-stable-release-recommended)
- [Option 2: Pre-release Version (Testing/Development)](#option-2-pre-release-version-testingdevelopment)

2. [Manual Docker Build Commands](#manual-docker-build-commands)

- [Stable Release](#stable-release)
- [Pre-release Version](#pre-release-version)

3. [Docker Compose Configuration](#docker-compose-configuration)

- [Default docker-compose.yml (Stable)](#default-docker-composeyml-stable)
- [Pre-release docker-compose.yml](#pre-release-docker-composeyml)

4. [Check Status](#check-status)
5. [Version Information](#version-information)

- [Stable Release](#stable-release-1)
- [Pre-release Version](#pre-release-version-1)

6. [Switching Between Versions](#switching-between-versions)

- [From Stable to Pre-release](#from-stable-to-pre-release)
- [From Pre-release to Stable](#from-pre-release-to-stable)

7. [How It Works](#how-it-works)
8. [Release Architecture Support](#release-architecture-support)
9. [Configuration](#configuration)

- [Method 1: Direct Config File Editing (Recommended)](#method-1-direct-config-file-editing-recommended)
- [Method 2: Environment Variables (Limited Options)](#method-2-environment-variables-limited-options)
- [Method 3: Edit Inside Container](#method-3-edit-inside-container)

10. [Viewing Logs](#viewing-logs)

- [1. Container Startup Logs (Minimal)](#1-container-startup-logs-minimal)
- [2. Application Debug Logs (Everything Else)](#2-application-debug-logs-everything-else)
- [Log Configuration](#log-configuration)
- [Accessing Log Files](#accessing-log-files)

11. [Health Monitoring](#health-monitoring)
12. [Management Commands](#management-commands)

- [Basic Operations](#basic-operations)
- [Updates and Maintenance](#updates-and-maintenance)
- [Database Operations](#database-operations)
- [Troubleshooting](#troubleshooting)

13. [Security Considerations](#security-considerations)
14. [Troubleshooting](#troubleshooting-1)

- [Common Issues](#common-issues)
- [Architecture issues](#architecture-issues)
- [Config changes not taking effect](#config-changes-not-taking-effect)
- [Can't connect to relay](#cant-connect-to-relay)
- [Pre-release not found](#pre-release-not-found)
- [Need to see what's happening](#need-to-see-whats-happening)

## Quick Start

### Option 1: Stable Release (Recommended)

1. **Create project directory:**

```bash
mkdir grain-docker
cd grain-docker
```

2. **Download files:**

```bash
# Download Dockerfile and docker-compose.yml
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/Dockerfile
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/docker-compose.yml
```

3. **Start relay:**

```bash
docker compose up -d
```

### Option 2: Pre-release Version (Testing/Development)

1. **Create project directory:**

```bash
mkdir grain-docker
cd grain-docker
```

2. **Download files:**

```bash
# Download both Dockerfiles and docker-compose.yml
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/Dockerfile
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/Dockerfile-prerelease
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/docker-compose.yml
```

3. **Start relay with pre-release:**

```bash
# Build and start using pre-release Dockerfile
docker compose -f docker-compose.yml up -d --build grain-prerelease
```

**OR** modify your `docker-compose.yml` to use the pre-release Dockerfile:

```yaml
services:
  grain:
    build:
      context: .
      dockerfile: Dockerfile-prerelease
    # ... rest of your config
```

Then run normally:

```bash
docker compose up -d
```

## Manual Docker Build Commands

If you prefer to build manually instead of using docker-compose:

### Stable Release

```bash
# Build stable version
docker build -t grain:stable .

# Run stable version
docker run -d -p 8181:8181 --name grain-relay grain:stable
```

### Pre-release Version

```bash
# Build pre-release version
docker build -f Dockerfile-prerelease -t grain:prerelease .

# Run pre-release version
docker run -d -p 8181:8181 --name grain-relay grain:prerelease
```

## Docker Compose Configuration

### Default docker-compose.yml (Stable)

```yaml
version: "3.8"

services:
  grain:
    build:
      context: .
      dockerfile: Dockerfile # Uses stable release
    ports:
      - "8181:8181"
    environment:
      - GRAIN_ENV=production
      - MONGO_URI=mongodb://mongo:27017/grain
      - LOG_LEVEL=info
      - SERVER_PORT=8181
    depends_on:
      - mongo
    restart: unless-stopped
    healthcheck:
      test:
        [
          "CMD",
          "wget",
          "--no-verbose",
          "--tries=1",
          "--spider",
          "http://localhost:8181/",
        ]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s

  mongo:
    image: mongo:7
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    restart: unless-stopped

volumes:
  mongo_data:
```

### Pre-release docker-compose.yml

```yaml
version: "3.8"

services:
  grain:
    build:
      context: .
      dockerfile: Dockerfile-prerelease # Uses pre-release
    ports:
      - "8181:8181"
    environment:
      - GRAIN_ENV=development # Note: development environment
      - MONGO_URI=mongodb://mongo:27017/grain
      - LOG_LEVEL=debug # More verbose logging for testing
      - SERVER_PORT=8181
    depends_on:
      - mongo
    restart: unless-stopped
    healthcheck:
      test:
        [
          "CMD",
          "wget",
          "--no-verbose",
          "--tries=1",
          "--spider",
          "http://localhost:8181/",
        ]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s

  mongo:
    image: mongo:7
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    restart: unless-stopped

volumes:
  mongo_data:
```

## Check Status

4. **Check status:**

```bash
docker compose ps
docker compose logs grain
```

Your relay is now running at:

- WebSocket: `ws://localhost:8181`
- Web: `http://localhost:8181`

## Version Information

### Stable Release

- **Source**: Uses `releases/latest` from GitHub API
- **Stability**: Production-ready, thoroughly tested
- **Update frequency**: Major/minor releases
- **Recommended for**: Production deployments

### Pre-release Version

- **Source**: Uses latest tagged pre-release from GitHub API
- **Stability**: Testing/development, may contain bugs
- **Update frequency**: Release candidates, beta versions
- **Recommended for**: Testing new features, development
- **Fallback**: Automatically falls back to stable if no pre-release exists

## Switching Between Versions

### From Stable to Pre-release

```bash
# Stop current container
docker compose down

# Build and start with pre-release
docker build -f Dockerfile-prerelease -t grain:prerelease .
docker run -d -p 8181:8181 --name grain-relay grain:prerelease
```

### From Pre-release to Stable

```bash
# Stop current container
docker compose down

# Build and start with stable
docker build -f Dockerfile -t grain:stable .
docker run -d -p 8181:8181 --name grain-relay grain:stable
```

## How It Works

Both Dockerfiles:

- Automatically detect your system architecture (amd64/arm64)
- Download the appropriate release binary from GitHub releases
- Extract the binary and www assets into a minimal Alpine Linux container
- Create a secure non-root user environment
- GRAIN automatically creates configuration files from embedded examples on first startup

**Key Differences:**

- **Dockerfile**: Downloads from `/releases/latest` (stable releases only)
- **Dockerfile-prerelease**: Downloads from `/releases` and filters for `prerelease: true`

## Release Architecture Support

Both Docker builds automatically select the correct binary based on your host architecture:

- **x86_64 systems** → Downloads `grain-linux-amd64.tar.gz`
- **ARM64 systems** → Downloads `grain-linux-arm64.tar.gz`

This eliminates the need for compilation during Docker build, making the process faster and more reliable.

## Configuration

GRAIN uses an embedded configuration system that automatically creates config files from built-in examples on first startup. This eliminates the need for external config management.

### Method 1: Direct Config File Editing (Recommended)

To customize configuration files:

1. **Extract configs from running container:**

```bash
docker cp grain-relay:/app/config.yml .
docker cp grain-relay:/app/relay_metadata.json .
docker cp grain-relay:/app/whitelist.yml .
docker cp grain-relay:/app/blacklist.yml .
```

2. **Edit files locally:**

```bash
nano config.yml          # Server settings, rate limits, logging
nano relay_metadata.json # Relay info (name, description, contact)
nano whitelist.yml       # Allowed users, domains, event types
nano blacklist.yml       # Banned content and escalation policies
```

3. **Apply changes back to container:**

```bash
docker cp config.yml grain-relay:/app/config.yml
docker cp relay_metadata.json grain-relay:/app/relay_metadata.json
docker cp whitelist.yml grain-relay:/app/whitelist.yml
docker cp blacklist.yml grain-relay:/app/blacklist.yml
```

**Note**: GRAIN automatically detects config changes and hot-reloads - no container restart needed!

### Method 2: Environment Variables (Limited Options)

For basic settings, you can use these **four** environment variables in docker-compose.yml:

```yaml
services:
  grain:
    # ... other settings ...
    environment:
      - GRAIN_ENV=production # Environment name (use 'development' for pre-release)
      - MONGO_URI=mongodb://mongo:27017/grain # MongoDB connection
      - LOG_LEVEL=info # Log level: debug, info, warn, error (use 'debug' for pre-release)
      - SERVER_PORT=8181 # Server port number
```

**Note**: Only these 4 environment variables are supported. For all other settings (rate limits, authentication, resource limits, etc.), use Method 1.

### Method 3: Edit Inside Container

For quick changes, you can edit directly inside the container:

```bash
# Access container shell
docker exec -it grain-relay sh

# Edit configs with vi
vi config.yml
vi relay_metadata.json
vi whitelist.yml
vi blacklist.yml

# Exit container
exit
```

## Viewing Logs

GRAIN uses two distinct logging systems:

### 1. Container Startup Logs (Minimal)

Docker container logs **only** show basic startup information:

```bash
# View startup logs
docker compose logs grain

# Example output:
# grain-relay | Server is running on http://localhost:8181
```

That's it - container logs are minimal by design.

### 2. Application Debug Logs (Everything Else)

**ALL** application logging goes to `debug.log` inside the container:

```bash
# View current debug log
docker exec grain-relay cat /app/debug.log

# Follow debug log in real-time (THIS IS YOUR MAIN LOG)
docker exec grain-relay tail -f /app/debug.log

# Copy debug log to host system
docker cp grain-relay:/app/debug.log ./debug.log
```

The debug log contains:

- Configuration loading
- MongoDB connections
- WebSocket connections/disconnections
- Event processing
- Rate limiting actions
- Whitelist/blacklist decisions
- Errors and warnings
- All other application activity

### Log Configuration

The debug log behavior is controlled by your `config.yml` logging section:

```yaml
logging:
  level: "info" # Log level: debug, info, warn, error
  file: "debug" # Log file name (becomes debug.log or debug.json)
  max_log_size_mb: 10 # Max size before rotation
  structure: false # false = pretty logs, true = JSON format
  check_interval_min: 10 # Check for rotation every 10 minutes
  backup_count: 2 # Keep 2 backup files
  suppress_components: # Reduce noise from these components
    - "util"
    - "conn-manager"
```

### Accessing Log Files

```bash
# List all log files in container
docker exec grain-relay ls -la *.log* *.json*

# View log rotation files
docker exec grain-relay ls -la debug.log*

# Copy all logs to host
docker cp grain-relay:/app/debug.log ./
docker cp grain-relay:/app/debug.log.bak1 ./
docker cp grain-relay:/app/debug.log.bak2 ./
```

## Health Monitoring

The container includes built-in health checks:

```bash
# Check health status
docker compose ps

# View health check logs
docker inspect grain-relay --format='{{.State.Health}}'

# Manual health check
curl http://localhost:8181/
```

## Management Commands

### Basic Operations

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# Restart services
docker compose restart

# View service status
docker compose ps
```

### Updates and Maintenance

```bash
# Update to latest stable release
docker compose down
docker compose build --no-cache
docker compose up -d

# Update to latest pre-release
docker compose down
docker build -f Dockerfile-prerelease --no-cache -t grain:prerelease .
docker compose up -d

# View build logs
docker compose build --no-cache --progress=plain

# Clean up old images
docker image prune -f
```

### Database Operations

```bash
# Backup database
docker compose exec mongo mongodump --db grain --archive | gzip > backup-$(date +%Y%m%d).gz

# Restore database
gunzip < backup-YYYYMMDD.gz | docker compose exec -T mongo mongorestore --db grain --archive

# Access MongoDB shell
docker compose exec mongo mongosh grain

# View database stats
docker compose exec mongo mongosh grain --eval "db.stats()"
```

### Troubleshooting

```bash
# Check container resource usage
docker stats grain-relay grain-mongo

# Access container shell for debugging
docker exec -it grain-relay sh

# Check file permissions
docker exec grain-relay ls -la /app/

# Check release download and extraction (stable)
docker compose logs grain | grep -i "download\|extract\|version"

# Check pre-release download (when using Dockerfile-prerelease)
docker logs grain-relay | grep -i "pre-release\|download\|extract\|version"

# Reset configs to defaults (removes and recreates from embedded)
docker exec grain-relay rm config.yml relay_metadata.json whitelist.yml blacklist.yml
docker compose restart grain
```

## Security Considerations

The Docker setup includes several security best practices:

- **Non-root user**: Container runs as user `grain` (UID 1001)
- **Minimal base image**: Uses Alpine Linux for smaller attack surface
- **Pre-built binaries**: Uses official release binaries instead of building from source
- **Health checks**: Automated monitoring of service health
- **Resource limits**: Can be configured in docker-compose.yml

## Troubleshooting

### Common Issues

**Container won't start:**

```bash
# Check build logs
docker compose build --no-cache --progress=plain

# Check for port conflicts
netstat -an | grep :8181

# Check release download issues
docker compose logs grain | grep -E "(error|fail|download)"
```

**Architecture issues:**

```bash
# Check your system architecture
uname -m

# Force rebuild with verbose output
docker compose build --no-cache --progress=plain
```

**Config changes not taking effect:**

```bash
# Check if hot-reload is working (in debug.log, not container logs!)
docker exec grain-relay tail -f /app/debug.log | grep -i "config"

# Force restart if needed
docker compose restart grain
```

**Can't connect to relay:**

```bash
# Test connectivity
curl http://localhost:8181
curl -H "Accept: application/nostr+json" http://localhost:8181

# Check firewall
sudo ufw status

# Check actual application logs for errors
docker exec grain-relay tail -100 /app/debug.log
```

**Pre-release not found:**

```bash
# Check if pre-releases exist
curl -s https://api.github.com/repos/0ceanslim/grain/releases | jq '.[] | select(.prerelease == true) | .tag_name'

# Check build logs for fallback to stable
docker logs grain-relay | grep -i "no pre-release found"
```

**Need to see what's happening:**

Remember: `docker compose logs` only shows startup messages. For actual application activity:

```bash
# This is where all the action is:
docker exec grain-relay tail -f /app/debug.log
```

The build process automatically downloads and extracts the appropriate release binary with all assets based on your chosen Dockerfile!
