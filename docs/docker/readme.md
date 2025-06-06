# GRAIN Docker Setup UNTESTED WIP

Run GRAIN relay with Docker using the latest stable release source code.

## Quick Start

1. **Create project directory:**

```bash
mkdir grain-docker
cd grain-docker
```

2. **Download files:**

```bash
# Download Dockerfile and docker-compose.yml
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docker/Dockerfile
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docker/docker-compose.yml
```

3. **Start relay:**

```bash
docker-compose up -d
```

4. **Check status:**

```bash
docker-compose ps
docker-compose logs grain
```

Your relay is now running at:

- WebSocket: `ws://localhost:8181`
- Web: `http://localhost:8181`

## How It Works

The Dockerfile:

- Downloads the latest release source code from GitHub
- Builds the GRAIN binary using Go 1.23
- Downloads the `www.zip` frontend assets from the release
- Creates a minimal Alpine Linux container with just the binary and assets

## Configuration

To customize your relay:

1. **Extract configs:**

```bash
docker cp grain-relay:/app/config.yml .
docker cp grain-relay:/app/relay_metadata.json .
docker cp grain-relay:/app/whitelist.yml .
docker cp grain-relay:/app/blacklist.yml .
```

2. **Edit files:**

```bash
nano config.yml          # Server settings, rate limits, logging
nano relay_metadata.json # Relay info (name, description, contact)
nano whitelist.yml       # Allowed users, domains, event types
nano blacklist.yml       # Banned content and escalation policies
```

3. **Apply changes:**

```bash
docker cp config.yml grain-relay:/app/config.yml
docker cp relay_metadata.json grain-relay:/app/relay_metadata.json
docker cp whitelist.yml grain-relay:/app/whitelist.yml
docker cp blacklist.yml grain-relay:/app/blacklist.yml
```

GRAIN automatically detects config changes and restarts - no need to restart the container!

## Viewing Logs

**Application debug logs** (detailed program logs):

```bash
# View current debug log
docker exec grain-relay cat /app/debug.log

# Follow debug log in real-time
docker exec grain-relay tail -f /app/debug.log

# Copy debug log to host
docker cp grain-relay:/app/debug.log ./debug.log
```

The `debug.log` file contains detailed application logs set by your logging configuration.

## Management Commands

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose down

# Update to latest release
docker-compose down
docker-compose build --no-cache
docker-compose up -d

# Backup database
docker-compose exec mongo mongodump --db grain --archive | gzip > backup.gz

# Restore database
gunzip < backup.gz | docker-compose exec -T mongo mongorestore --db grain --archive
```

The build process automatically downloads and compiles the latest tagged release!
