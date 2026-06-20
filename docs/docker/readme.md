# GRAIN Docker Setup

Run a GRAIN relay with Docker — zero-dependency, no external database. For most deployments you only need to start the container and then **set everything up from the web dashboard**; no config-file editing required.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Set up your relay (web dashboard)](#set-up-your-relay-web-dashboard)
3. [Docker Compose Configuration](#docker-compose-configuration)
4. [Data Persistence](#data-persistence)
5. [Advanced: environment variables & config files](#advanced-environment-variables--config-files)
6. [Viewing Logs](#viewing-logs)
7. [Management Commands](#management-commands)
8. [Troubleshooting](#troubleshooting)

## Quick Start

### 1. Create a project directory
```bash
mkdir grain-docker
cd grain-docker
```

### 2. Download the Docker files
```bash
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/Dockerfile
curl -O https://raw.githubusercontent.com/0ceanslim/grain/main/docs/docker/docker-compose.yml
```

### 3. Start the relay
```bash
docker compose up -d
```

Your relay is now running:
- WebSocket: `ws://localhost:8181`
- Web dashboard: `http://localhost:8181`

### 4. Set it up in the browser

Open `http://localhost:8181` and finish setup from the web UI — see the next section.

---

## Set up your relay (web dashboard)

**This is all most operators need.** GRAIN is configured live from its web dashboard — no YAML editing, no restarts.

1. Open `http://localhost:8181`. A banner shows the relay is **unclaimed**.
2. Go to **`/setup`** (or click the banner), **sign in with your Nostr key**, and **claim ownership**.
3. Open **`/admin`** and configure everything from forms: relay identity (name, icon, description, policy URLs), rate limits, whitelist / blacklist, event purging, time constraints, NIP-42 AUTH, resource limits, and more. Changes apply live.

That's the whole flow — start the container, claim, and run the relay from the browser. (To pre-assign the owner without visiting `/setup` — handy for scripted deployments — set `GRAIN_OWNER_PUBKEY`; see [Advanced](#advanced-environment-variables--config-files).)

---

## Docker Compose Configuration

The shipped `docker-compose.yml` is all you need — there's no separate database service to run:

```yaml
services:
  grain:
    build: . # Uses the Dockerfile in current directory
    container_name: grain-relay
    ports:
      - "8181:8181"
    volumes:
      # GRAIN writes config + the LMDB store + logs under /home/grain/.grain.
      # A named volume here persists the relay's full state across recreation.
      - grain_data:/home/grain/.grain
    restart: unless-stopped

volumes:
  grain_data:
```

---

## Data Persistence

GRAIN uses an embedded **nostrdb** engine. It is critical to use a Docker volume to persist your data, otherwise your database and configuration will be lost when the container is removed.

The default `docker-compose.yml` mounts a named volume on `/home/grain/.grain`, which is GRAIN's data directory inside the container — it holds `config.yml`, `blacklist.yml`, `whitelist.yml`, `relay_metadata.json`, the LMDB store under `data/`, and the runtime log file `debug.log`. Mounting the entire directory persists the relay's full operational state across container recreation.

---

## Advanced: environment variables & config files

For development, scripted deployments, or fine-grained control, you can override settings outside the dashboard.

### Environment variables

Set these under `environment:` in `docker-compose.yml`:

| Variable | Effect |
|---|---|
| `GRAIN_OWNER_PUBKEY` | pre-assign the relay owner (hex or npub) so you skip the `/setup` claim |
| `SERVER_PORT` | listen port inside the container (default `8181`) |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error` |
| `NDB_PATH` | override the nostrdb data path |
| `GRAIN_DATA_DIR` | override the data directory (config + database + logs) |

### Editing config files directly

Bind a host directory to the data directory to edit the YAML by hand:

```yaml
services:
  grain:
    volumes:
      - ./my-grain-config:/home/grain/.grain
```

Edit `config.yml`, `whitelist.yml`, `blacklist.yml`, etc. on the host. GRAIN **hot-reloads** config, so changes apply without restarting the container. The host directory must be writable by uid/gid `1001:1001` (the non-root `grain` user inside the container); GRAIN populates it with default files on first run if it's empty.

---

## Viewing Logs

GRAIN logs to both stdout (minimal) and a structured log file.

### Container Logs (Startup)
```bash
docker compose logs -f grain
```

By default the relay only writes to its log file inside the container, so `docker logs` shows just the startup line. To mirror the live log to stdout (so `docker compose logs -f grain` shows everything in real time), set `stdout: true` under `logging:` in `config.yml`:

```yaml
logging:
  stdout: true
```

The file sink is unaffected — the canonical `debug.log` continues to be written; stdout just gets a copy.

### Application Logs (Detailed)
```bash
# View real-time application activity from the file
docker exec grain-relay tail -f /home/grain/.grain/debug.log
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
