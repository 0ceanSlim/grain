# GRAIN Docker Setup

Run GRAIN relay with Docker using either the latest stable release or pre-release binaries. Starting with v0.5.0, GRAIN is zero-dependency and does not require an external database like MongoDB.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Docker Compose Configuration](#docker-compose-configuration)
3. [Data Persistence](#data-persistence)
4. [Manual Docker Build Commands](#manual-docker-build-commands)
5. [Configuration](#configuration)
6. [Viewing Logs](#viewing-logs)
7. [Management Commands](#management-commands)
8. [Security Considerations](#security-considerations)
9. [Troubleshooting](#troubleshooting)

## Quick Start

### 1. Create project directory
```bash
mkdir grain-docker
cd grain-docker
```

### 2. Download Docker files
```bash
# Download Dockerfile and docker-compose.yml
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/Dockerfile
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/docker-compose.yml
```

### 3. Start relay
```bash
docker compose up -d
```

Your relay is now running at:
- WebSocket: `ws://localhost:8181`
- Web: `http://localhost:8181`

---

## Docker Compose Configuration

Starting with v0.5.0, the `docker-compose.yml` is significantly simplified as it no longer requires a MongoDB service.

### Default `docker-compose.yml`

```yaml
version: "3.8"

services:
  grain:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8181:8181"
    environment:
      - GRAIN_ENV=production
      - LOG_LEVEL=info
    volumes:
      - grain_data:/app/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8181/"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  grain_data:
```

---

## Data Persistence

GRAIN v0.5.0 uses an embedded **nostrdb** engine. It is critical to use a Docker volume to persist your data, otherwise your database and configurations will be lost when the container is removed.

The default Dockerfile is configured to store data in `/app/data` (which maps to your platform's data directory inside the container).

---

## Configuration

GRAIN automatically creates default configuration files on first startup.

### Method 1: Environment Variables
You can set basic options directly in `docker-compose.yml`:
- `GRAIN_ENV`: `production` or `development`
- `LOG_LEVEL`: `debug`, `info`, `warn`, `error`
- `SERVER_PORT`: Internal port (default 8181)

### Method 2: Volume Mapping (Recommended)
Map a local directory to the container's data path to manage config files easily:

```yaml
services:
  grain:
    volumes:
      - ./my-grain-config:/app/data
```

You can then edit `config.yml`, `whitelist.yml`, etc., directly on your host machine. GRAIN supports **hot-reloading**, so changes are applied instantly without restarting the container.

---

## Viewing Logs

GRAIN logs to both stdout (minimal) and a structured log file.

### Container Logs (Startup)
```bash
docker compose logs -f grain
```

### Application Logs (Detailed)
```bash
# View real-time application activity
docker exec grain-relay tail -f /app/data/logs/debug.log
```

---

## Management Commands

### Updates
To update to the latest version:
```bash
docker compose pull
docker compose up -d
```

### Database Maintenance
Since storage is embedded, you can perform maintenance via the GRAIN CLI:
```bash
# Check database stats
docker exec grain-relay ./grain --stats

# Physically delete an event
docker exec grain-relay ./grain --delete <event_id>
```

---

## Troubleshooting

### Architecture Mismatch
The Dockerfile automatically detects `amd64` or `arm64`. If the build fails, ensure your Docker version supports multi-platform builds or check your internet connectivity to GitHub Releases.

### Port Conflicts
If port 8181 is already in use, change the host mapping in `docker-compose.yml`:
```yaml
ports:
  - "9090:8181" # Map host 9090 to container 8181
```

### Database Locked
If the container crashes and won't restart, ensure no other process is accessing the `data/` volume. LMDB (used by nostrdb) requires exclusive access to its lock files.

---

**GRAIN Docker Setup Complete!** 🌾
