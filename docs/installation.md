# GRAIN Installation Guide

Complete installation instructions for setting up your GRAIN relay server.

## Table of Contents

- [Overview](#overview)
- [System Requirements](#system-requirements)
- [Method 1: Pre-built Binaries (Recommended)](#method-1-pre-built-binaries-recommended)
- [Method 2: Building from Source](#method-2-building-from-source)
- [Method 3: Docker Deployment](#method-3-docker-deployment)
- [Data Directory & Storage](#data-directory--storage)
- [First Run Setup](#first-run-setup)
- [Configuration](#configuration)
- [Service Installation](#service-installation)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

---

## Overview

GRAIN is a zero-dependency, single-binary Nostr relay and client library. Starting with v0.5.0, it no longer requires an external database like MongoDB. All storage is handled by an embedded, high-performance engine (**nostrdb**).

GRAIN can be installed in three ways:

1. **Pre-built binaries** - Download and run (recommended for most users)
2. **Build from source** - Compile yourself for custom builds or unsupported platforms
3. **Docker containers** - Containerized deployment for modern infrastructure

---

## System Requirements

### Supported Platforms

| Platform | Architecture  | Pre-built Binary | Source Build | Docker |
| -------- | ------------- | ---------------- | ------------ | ------ |
| Linux    | x86_64        | ✅               | ✅           | ✅     |
| Linux    | ARM64         | ✅               | ✅           | ✅     |
| macOS    | x86_64        | ✅               | ✅           | ✅     |
| macOS    | ARM64 (M1/M2) | ✅               | ✅           | ✅     |
| Windows  | x86_64        | ✅               | ✅           | ✅     |

---

## Method 1: Pre-built Binaries (Recommended)

The fastest way to get GRAIN running is using pre-built binaries.

### Step 1: Download GRAIN

1. **Visit the releases page**: [https://github.com/0ceanslim/grain/releases](https://github.com/0ceanslim/grain/releases)

2. **Download for your platform**: Look for the archive corresponding to your OS and architecture.

### Step 2: Extract and Set Up

**Linux/macOS**:

```bash
# Extract the archive
tar -xzf grain-*.tar.gz

# Make executable
chmod +x grain

# Move to a directory in your PATH (optional)
sudo mv grain /usr/local/bin/
```

**Windows**:

1. Extract the `.zip` archive to a folder of your choice.
2. You will see `grain.exe`. This is the only file you need.

### Step 3: Verify Installation

```bash
grain --version
```

Expected output:
```
GRAIN v0.5.0
Go Relay Architecture for Implementing Nostr
```

---

## Method 2: Building from Source

Building from source requires a C compiler and Go 1.23+ due to the embedded `nostrdb` (C library).

### Prerequisites

- **Go 1.23+** - [Download Go](https://go.dev/dl/)
- **C Compiler** (gcc or clang)
- **Make** (for using build scripts)

### Step 1: Clone and Build

```bash
# Clone repository including submodules
git clone --recursive https://github.com/0ceanslim/grain.git
cd grain

# Build binary
go build -o grain .
```

On Windows, it is recommended to use **MSYS2** with the `mingw-w64-x86_64-gcc` package for the C components.

---

## Method 4: Docker Deployment

For containerized deployment, please refer to the [Docker Documentation](docker/README.md).

Since v0.5.0, the Docker image is significantly smaller as it no longer bundles MongoDB.

---

## Data Directory & Storage

GRAIN stores its database, logs, and configuration files in a platform-specific data directory.

### Default Locations

| OS      | Default Path                                   |
| ------- | ---------------------------------------------- |
| Linux   | `~/.grain/`                                    |
| macOS   | `~/Library/Application Support/grain/`          |
| Windows | `%APPDATA%\grain\`                             |

### Customizing the Data Directory

You can override the default location in two ways:

1. **CLI Flag**: `grain --data-dir /path/to/custom/dir`
2. **Environment Variable**: `export GRAIN_DATA_DIR=/path/to/custom/dir`

### Internal Structure

Inside the data directory, GRAIN maintains:
- `data/` - The **nostrdb** LMDB database files.
- `config.yml` - Main server configuration.
- `whitelist.yml` / `blacklist.yml` - Policy files.
- `relay_metadata.json` - NIP-11 information.
- `logs/` - Structured application logs.

---

## First Run Setup

### Step 1: Start GRAIN

Simply run the binary. On the first run, GRAIN will detect that no configuration exists and generate default files in your platform's data directory.

```bash
./grain
```

### Step 2: Migration (Optional)

If you are migrating from an older version of GRAIN that used MongoDB:

1. Export your MongoDB events to a JSONL file.
2. Import them into the new engine:
   ```bash
   ./grain --import events_export.jsonl
   ```

---

## Configuration

GRAIN uses hot-reloading for all configuration files. You can edit them while the server is running, and changes will be applied instantly.

- **`config.yml`**: Database paths, network settings, and rate limits.
- **`whitelist.yml`**: Allowed pubkeys and domains.
- **`blacklist.yml`**: Banned content and users.
- **`relay_metadata.json`**: Public info served via NIP-11.

See the [Configuration Guide](../docs/configuration.md) for full details.

---

## Service Installation

### Linux (systemd)

Create `/etc/systemd/system/grain.service`:

```ini
[Unit]
Description=GRAIN Nostr Relay
After=network.target

[Service]
Type=simple
User=grain
Group=grain
ExecStart=/usr/local/bin/grain
Restart=always
RestartSec=5

# Security settings
NoNewPrivileges=true
ProtectSystem=full

[Install]
WantedBy=multi-user.target
```

---

## Verification

### 1. HTTP Endpoint
```bash
curl -I http://localhost:8181
# Should return 200 OK
```

### 2. NIP-11 Information
```bash
curl -H "Accept: application/nostr+json" http://localhost:8181
```

### 3. WebSocket Connection
Use `websocat` to test the relay:
```bash
echo '["REQ","test",{"kinds":[1],"limit":1}]' | websocat ws://localhost:8181
```

---

## Troubleshooting

### CGO / Build Failures
If building from source fails, ensure you have a working C compiler and that `git submodules` were properly initialized (`git submodule update --init --recursive`).

### Permissions
Ensure the user running GRAIN has write access to the data directory (e.g., `~/.grain`).

### Database Map Size
If you see "MDB_MAP_FULL" in the logs, increase the `database.map_size_mb` in `config.yml`. The default is 4096 (4GB).

---

## Next Steps

1. **Configure your relay**: Edit `config.yml` and `relay_metadata.json`.
2. **Set up NIP-05**: Whitelist your pubkey using domain-based verification.
3. **Monitor**: Check the web dashboard at `http://localhost:8181`.

**Installation Complete!** 🌾
