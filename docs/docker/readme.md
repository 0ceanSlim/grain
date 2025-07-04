# GRAIN Docker Setup

Run GRAIN relay with Docker using the latest stable release binaries.

## Quick Start

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

4. **Check status:**

```bash
docker compose ps
docker compose logs grain
```

Your relay is now running at:

- WebSocket: `ws://localhost:8181`
- Web: `http://localhost:8181`

## How It Works

The Dockerfile:

- Automatically detects your system architecture (amd64/arm64)
- Downloads the latest pre-built release binary from GitHub releases
- Extracts the binary and www assets into a minimal Alpine Linux container
- Creates a secure non-root user environment
- GRAIN automatically creates configuration files from embedded examples on first startup

## Release Architecture Support

The Docker build automatically selects the correct binary based on your host architecture:

- **x86_64 systems** → Downloads `grain-linux-amd64.tar.gz`
- **ARM64 systems** → Downloads `grain-linux-arm64.tar.gz`

This eliminates the need for compilation during Docker build, making the process faster and more reliable.

## Configuration

GRAIN uses an embedded configuration system that automatically creates config files from built-in examples on first startup. This eliminates the need for external config management.

### Method 1: Environment Variables (Recommended for Production)

The easiest way to configure your relay is through environment variables in docker-compose.yml:

```yaml
services:
  grain:
    # ... other settings ...
    environment:
      - GRAIN_ENV=production
      - MONGO_URI=mongodb://mongo:27017/grain
      - LOG_LEVEL=info
      - SERVER_PORT=8181
      # Add other environment variables as needed
```

### Method 2: Direct Config File Editing

To customize configuration files beyond environment variables:

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

### Method 3: Edit Inside Container

For quick changes, you can edit directly inside the container:

```bash
# Access container shell
docker exec -it grain-relay sh

# Edit configs with vi (or your preferred editor)
vi config.yml
vi relay_metadata.json
vi whitelist.yml
vi blacklist.yml

# Exit container
exit
```

## Viewing Logs

GRAIN provides multiple logging options for monitoring and debugging.

### Application Debug Logs

GRAIN writes detailed application logs to `debug.log` inside the container:

```bash
# View current debug log
docker exec grain-relay cat /app/debug.log

# Follow debug log in real-time
docker exec grain-relay tail -f /app/debug.log

# Copy debug log to host system
docker cp grain-relay:/app/debug.log ./debug.log
```

### Container Logs

Docker container logs show startup and basic operational info:

```bash
# View container logs
docker compose logs grain

# Follow container logs in real-time
docker compose logs -f grain

# View last 50 lines
docker compose logs --tail=50 grain
```

### Log Configuration

The debug log behavior is controlled by your `config.yml` logging section:

```yaml
logging:
  level: "info" # Log level: debug, info, warn, error
  file: "debug" # Log file name (becomes debug.log)
  max_log_size_mb: 10 # Max size before rotation
  structure: false # false = pretty logs, true = JSON
  check_interval_min: 10 # Check for rotation every 10 minutes
  backup_count: 2 # Keep 2 backup files
  suppress_components: # Reduce noise from these components
    - "util"
    - "conn-manager"
```

### Accessing Log Files

```bash
# List all log files in container
docker exec grain-relay ls -la *.log*

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
# Update to latest release
docker compose down
docker compose build --no-cache
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

# Check release download and extraction
docker compose logs grain | grep -i "download\|extract\|version"

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

To add resource limits:

```yaml
services:
  grain:
    # ... other settings ...
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "0.5"
```

## Production Deployment

For production use, consider:

1. **Use external MongoDB**: Replace the included MongoDB with a managed service
2. **Add reverse proxy**: Use nginx/traefik for SSL termination
3. **Configure monitoring**: Set up log aggregation and metrics
4. **Backup strategy**: Implement automated database backups
5. **Resource limits**: Set appropriate CPU/memory limits
6. **Network security**: Use Docker networks and firewall rules

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
# Check if hot-reload is working
docker compose logs grain | grep -i "config"

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
```

The build process automatically downloads and extracts the latest stable release binary with all assets!
