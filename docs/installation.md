# GRAIN Installation Guide

Complete installation instructions for setting up your GRAIN relay server.

## Table of Contents

- [Overview](#overview)
- [System Requirements](#system-requirements)
- [Method 1: Pre-built Binaries (Recommended)](#method-1-pre-built-binaries-recommended)
- [Method 2: Building from Source](#method-2-building-from-source)
- [Method 3: Docker Deployment](#method-3-docker-deployment)
- [MongoDB Installation](#mongodb-installation)
- [First Run Setup](#first-run-setup)
- [Configuration](#configuration)
- [Service Installation](#service-installation)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

---

## Overview

GRAIN can be installed in three ways:

1. **Pre-built binaries** - Download and run (recommended for most users)
2. **Build from source** - Compile yourself for custom builds or unsupported platforms
3. **Docker containers** - Containerized deployment for modern infrastructure

All methods require MongoDB as a dependency.

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
| FreeBSD  | x86_64        | ❌               | ✅           | ✅     |

---

## Method 1: Pre-built Binaries (Recommended)

The fastest way to get GRAIN running is using pre-built binaries.

### Step 1: Download GRAIN

1. **Visit the releases page**: [https://github.com/0ceanslim/grain/releases](https://github.com/0ceanslim/grain/releases)

2. **Download for your platform**:

   ```bash
   # Linux x86_64
   wget https://github.com/0ceanslim/grain/releases/latest/download/grain-linux-amd64.tar.gz

   # Linux ARM64
   wget https://github.com/0ceanslim/grain/releases/latest/download/grain-linux-arm64.tar.gz

   # macOS x86_64
   wget https://github.com/0ceanslim/grain/releases/latest/download/grain-darwin-amd64.tar.gz

   # macOS ARM64 (M1/M2)
   wget https://github.com/0ceanslim/grain/releases/latest/download/grain-darwin-arm64.tar.gz

   # Windows x86_64
   # Download grain-windows-amd64.zip from the releases page
   ```

### Step 2: Extract and Set Up

**Linux/macOS**:

```bash
# Create installation directory (can be anywhere you prefer)
mkdir -p ~/grain-relay
cd ~/grain-relay

# Extract the archive
tar -xzf ~/Downloads/grain-*.tar.gz --strip-components=1

# Make executable
chmod +x grain

# Verify structure
ls -la
# Should show: grain (executable), www/ (directory)
```

**Windows**:

1. Create folder where you want GRAIN (e.g., `C:\grain\` or `C:\Users\YourName\grain\`)
2. Extract `grain-windows-amd64.zip` to that folder
3. Verify you have `grain.exe` and `www\` folder in the same directory

### Step 3: Add to PATH (Optional)

This step allows you to run `grain` from anywhere for CLI commands.

**Linux/macOS**:

```bash
# Add to your shell profile (choose one):
echo 'export PATH="$HOME/grain-relay:$PATH"' >> ~/.bashrc   # Bash
echo 'export PATH="$HOME/grain-relay:$PATH"' >> ~/.zshrc    # Zsh

# Reload shell
source ~/.bashrc  # or ~/.zshrc

# Or create a symbolic link
sudo ln -s ~/grain-relay/grain /usr/local/bin/grain
```

**Windows**:

1. Open System Properties → Advanced → Environment Variables
2. Add your GRAIN directory (e.g., `C:\grain\`) to the PATH variable
3. Or create a batch file in a directory already in PATH:

```batch
@echo off
cd /d "C:\grain"
grain.exe %*
```

### Step 4: Verify Installation

```bash
# From the GRAIN directory
./grain --version

# Or if added to PATH
grain --version

# Verify www directory structure
ls -la www/
```

Expected output:

```
GRAIN v0.4.0
Go Relay Architecture for Implementing Nostr

www/
├── static/
├── style/
└── views/
```

---

## Method 2: Building from Source

Build GRAIN yourself for custom configurations or unsupported platforms.

### Prerequisites

- **Go 1.21+** - [Download Go](https://go.dev/dl/)
- **Git** - For cloning the repository
- **Make** (optional) - For using build scripts

### Step 1: Install Go

**Linux (Ubuntu/Debian)**:

```bash
# Remove old Go versions
sudo rm -rf /usr/local/go

# Download and install Go 1.21+
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
```

**macOS**:

```bash
# Using Homebrew
brew install go

# Or download from https://go.dev/dl/
```

**Windows**:

1. Download installer from [https://go.dev/dl/](https://go.dev/dl/)
2. Run installer and follow prompts
3. Verify in Command Prompt: `go version`

### Step 2: Clone and Build

```bash
# Clone repository
git clone https://github.com/0ceanslim/grain.git
cd grain

# Download dependencies
go mod download

# Build binary
go build -o grain .
```

### Step 3: Set Up Built Binary

```bash
# Create your installation directory
mkdir -p ~/grain-relay
cd ~/grain-relay

# Copy built binary and www directory
cp ~/path/to/grain/grain .
cp -r ~/path/to/grain/www .

# Make executable (Linux/macOS)
chmod +x grain

# Verify structure
ls -la
# Should show: grain (executable), www/ (directory)
```

### Build Options

**Development Build**:

```bash
go build -tags dev -o grain .
```

**Production Build** (optimized):

```bash
go build -ldflags="-w -s" -o grain .
```

**Static Build** (for containers):

```bash
CGO_ENABLED=0 go build -a -ldflags="-w -s" -o grain .
```

---

## Method 3: Docker Deployment

For containerized deployment of GRAIN using Docker, please refer to the comprehensive [Docker Documentation](docker/README.md).

The Docker guide covers:

- Quick start with Docker Compose
- Official and custom Docker images
- Production deployment strategies
- Kubernetes configurations
- Security best practices
- Monitoring and logging setups

---

## MongoDB Installation

GRAIN requires MongoDB for storing Nostr events and relay data.

### Option 1: Package Manager Installation

**Ubuntu/Debian**:

```bash
# Import MongoDB public key
curl -fsSL https://pgp.mongodb.com/server-7.0.asc | sudo gpg --dearmor -o /usr/share/keyrings/mongodb-server-7.0.gpg

# Add MongoDB repository
echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/7.0 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-7.0.list

# Update and install
sudo apt update
sudo apt install -y mongodb-org

# Start and enable service
sudo systemctl start mongod
sudo systemctl enable mongod

# Verify installation
sudo systemctl status mongod
mongosh --eval "db.runCommand({ connectionStatus: 1 })"
```

**CentOS/RHEL/Fedora**:

```bash
# Create repository file
sudo tee /etc/yum.repos.d/mongodb-org-7.0.repo << EOF
[mongodb-org-7.0]
name=MongoDB Repository
baseurl=https://repo.mongodb.org/yum/redhat/\$releasever/mongodb-org/7.0/x86_64/
gpgcheck=1
enabled=1
gpgkey=https://www.mongodb.org/static/pgp/server-7.0.asc
EOF

# Install MongoDB
sudo dnf install -y mongodb-org

# Start and enable service
sudo systemctl start mongod
sudo systemctl enable mongod
```

**macOS**:

```bash
# Using Homebrew
brew tap mongodb/brew
brew install mongodb-community@7.0

# Start service
brew services start mongodb/brew/mongodb-community

# Or start manually
mongod --config /usr/local/etc/mongod.conf
```

**Windows**:

1. Download MongoDB Community Server from [mongodb.com](https://www.mongodb.com/try/download/community)
2. Run the installer (.msi file)
3. Choose "Complete" installation
4. Install MongoDB as a Windows Service
5. Start MongoDB service from Services panel

### Option 2: Docker MongoDB

```bash
# Run MongoDB in Docker
docker run -d \
  --name grain-mongo \
  -p 27017:27017 \
  -v mongo-data:/data/db \
  --restart unless-stopped \
  mongo:7.0

# Verify connection
docker exec grain-mongo mongosh --eval "db.runCommand({ connectionStatus: 1 })"
```

### Option 3: MongoDB Atlas (Cloud)

1. Create account at [mongodb.com/atlas](https://www.mongodb.com/atlas)
2. Create a free cluster
3. Configure network access (add your IP)
4. Create database user
5. Get connection string
6. Update GRAIN config with Atlas URI:

```yaml
# config.yml
mongodb:
  uri: "mongodb+srv://username:password@cluster.mongodb.net/grain?retryWrites=true&w=majority"
  database: "grain"
```

### MongoDB Configuration

**Basic Configuration** (`/etc/mongod.conf`):

```yaml
storage:
  dbPath: /var/lib/mongodb
  journal:
    enabled: true

systemLog:
  destination: file
  logAppend: true
  path: /var/log/mongodb/mongod.log

net:
  port: 27017
  bindIp: 127.0.0.1

processManagement:
  timeZoneInfo: /usr/share/zoneinfo
```

**Performance Optimization**:

```yaml
storage:
  wiredTiger:
    engineConfig:
      cacheSizeGB: 2 # Adjust based on available RAM
    collectionConfig:
      blockCompressor: snappy
    indexConfig:
      prefixCompression: true
```

---

## First Run Setup

Initial configuration and startup process.

### Step 1: Navigate to GRAIN Directory

```bash
# Navigate to your GRAIN installation directory
cd ~/grain-relay

# Or wherever you extracted GRAIN
# cd /path/to/your/grain/directory

# Verify you have both grain executable and www directory
ls -la
# Should show: grain (or grain.exe), www/
```

### Step 2: Start GRAIN

**First run (creates default configs)**:

```bash
# Start GRAIN from its directory
./grain

# On Windows
grain.exe

# Or if added to PATH, from any directory
grain
```

**Expected output in debug.log**:

```
Server configuration not found, creating from example: config.yml
Whitelist configuration not found, creating from example: whitelist.yml
Blacklist configuration not found, creating from example: blacklist.yml
Relay metadata not found, creating from example: relay_metadata.json

[INFO] [main] GRAIN relay server starting
[INFO] [mongo] Connected to MongoDB successfully
[INFO] [main] HTTP server started address=:8181
Server is running on http://localhost:8181
```

### Step 3: Verify Installation

**Check web interface**:

```bash
# Open in browser or test with curl
curl http://localhost:8181

# Check NIP-11 endpoint
curl -H "Accept: application/nostr+json" http://localhost:8181
```

**Test WebSocket connection**:

```bash
# Using websocat (install: cargo install websocat)
echo '["REQ","test",{"kinds":[1],"limit":1}]' | websocat ws://localhost:8181

# Expected response: ["EOSE","test"]
```

### Step 4: Stop GRAIN

```bash
# Ctrl+C to stop gracefully
# Or send SIGTERM
kill -TERM $(pgrep grain)
```

---

## Configuration

Customize GRAIN for your specific needs.

### Generated Configuration Files

After first run, your GRAIN directory will contain:

```
~/grain-relay/
├── grain                   # Executable (grain.exe on Windows)
├── www/                    # Web interface files
├── config.yml              # Main server configuration
├── whitelist.yml           # User and content allowlists
├── blacklist.yml           # User and content blocklists
├── relay_metadata.json     # Public relay information
└── debug.log               # Application logs (created after first run)
```

### Essential Configuration Changes

**1. Relay Metadata** (`relay_metadata.json`):

```json
{
  "name": "🌾 My GRAIN Relay",
  "description": "A community Nostr relay",
  "pubkey": "your_pubkey_here",
  "contact": "admin@yourdomain.com",
  "supported_nips": [1, 7, 9, 11, 19, 42, 55, 65],
  "software": "https://github.com/0ceanslim/grain",
  "version": "0.4.0"
}
```

**2. Database Connection** (`config.yml`):

```yaml
mongodb:
  uri: "mongodb://localhost:27017/"
  database: "grain"
```

**3. Server Settings** (`config.yml`):

```yaml
server:
  port: ":8181" # Change port if needed
  read_timeout: 60
  write_timeout: 20
  idle_timeout: 1200
```

**4. Client Settings**: GRAIN will use these default relays for the web client:

```
- wss://relay.damus.io
- wss://nos.lol
- wss://relay.nostr.band
```

### Advanced Configuration

For detailed configuration options including:

- Complete server settings
- Rate limiting and resource management
- Authentication and access control
- Event purging and retention policies
- Performance tuning for different use cases

See the comprehensive [Configuration Guide](../docs/configuration.md).

---

## Service Installation

Run GRAIN as a system service for production deployments.

### Linux (systemd)

**Create service file** (`/etc/systemd/system/grain.service`):

```ini
[Unit]
Description=GRAIN Nostr Relay
After=network.target mongod.service
Wants=mongod.service

[Service]
Type=simple
User=grain
Group=grain
WorkingDirectory=/home/grain/grain-relay
ExecStart=/home/grain/grain-relay/grain
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/home/grain/grain-relay

[Install]
WantedBy=multi-user.target
```

**Install service**:

```bash
# Create grain user
sudo useradd -r -m -s /bin/false grain

# Set up GRAIN directory for service user
sudo mkdir -p /home/grain/grain-relay
sudo cp -r ~/grain-relay/* /home/grain/grain-relay/
sudo chown -R grain:grain /home/grain/grain-relay
sudo chmod +x /home/grain/grain-relay/grain

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable grain
sudo systemctl start grain

# Check status
sudo systemctl status grain
sudo journalctl -u grain -f
```

### macOS (launchd)

**Create plist file** (`~/Library/LaunchAgents/com.grain.relay.plist`):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.grain.relay</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/yourusername/grain-relay/grain</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/Users/yourusername/grain-relay</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/yourusername/grain-relay/grain.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/yourusername/grain-relay/grain.error.log</string>
</dict>
</plist>
```

**Load service**:

```bash
# Load and start service
launchctl load ~/Library/LaunchAgents/com.grain.relay.plist

# Check status
launchctl list | grep grain

# Stop service
launchctl unload ~/Library/LaunchAgents/com.grain.relay.plist
```

### Windows Service

Use [NSSM](https://nssm.cc/) (Non-Sucking Service Manager):

```cmd
# Download and install NSSM
# https://nssm.cc/download

# Install GRAIN as service (adjust path to your GRAIN directory)
nssm install GRAIN "C:\grain\grain.exe"
nssm set GRAIN AppDirectory "C:\grain"
nssm set GRAIN AppStdout "C:\grain\grain.log"
nssm set GRAIN AppStderr "C:\grain\grain.error.log"

# Start service
net start GRAIN

# Check status
sc query GRAIN
```

---

## Verification

Confirm your GRAIN installation is working correctly.

### Health Checks

**1. Process Check**:

```bash
# Check if GRAIN is running
ps aux | grep grain
# or
pgrep grain
```

**2. Port Check**:

```bash
# Verify port is listening
netstat -tlnp | grep 8181
# or
ss -tlnp | grep 8181
```

**3. HTTP Endpoint**:

```bash
# Test web interface
curl -I http://localhost:8181

# Expected: HTTP/1.1 200 OK
```

**4. NIP-11 Compliance**:

```bash
# Test relay info endpoint
curl -H "Accept: application/nostr+json" http://localhost:8181 | jq .

# Should return relay metadata JSON
```

**5. WebSocket Functionality**:

```bash
# Test WebSocket (requires websocat: cargo install websocat)
echo '["REQ","health-check",{"kinds":[1],"limit":1}]' | websocat ws://localhost:8181

# Should return: ["EOSE","health-check"]
```

### Database Verification

**MongoDB Connection**:

```bash
# Connect to MongoDB
mongosh grain

# Check collections
show collections

# Should show: events_0, events_1, events_3, etc.
```

**Event Storage Test**:

```bash
# Check for any stored events
mongosh grain --eval "db.events_1.countDocuments()"

# Should return: number of kind 1 events stored
```

### Log Analysis

**Check logs for errors**:

```bash
# View recent logs (from GRAIN directory)
tail -f debug.log

# Look for error patterns
grep -i error debug.log
grep -i failed debug.log
```

**Expected log entries**:

```
[INFO] [main] GRAIN relay server starting
[INFO] [mongo] Connected to MongoDB successfully
[INFO] [config] Configuration loaded successfully
[INFO] [main] HTTP server started address=:8181
```

---

## Troubleshooting

Common issues and solutions during installation.

### MongoDB Connection Issues

**Problem**: `Failed to connect to MongoDB`

**Solutions**:

```bash
# 1. Check if MongoDB is running
sudo systemctl status mongod

# 2. Test connection manually
mongosh --eval "db.runCommand({ connectionStatus: 1 })"

# 3. Check MongoDB logs
sudo tail -f /var/log/mongodb/mongod.log

# 4. Verify MongoDB configuration
sudo cat /etc/mongod.conf

# 5. Check firewall
sudo ufw status
sudo iptables -L
```

### Port Already in Use

**Problem**: `bind: address already in use`

**Solutions**:

```bash
# 1. Find what's using the port
sudo netstat -tlnp | grep 8181
sudo ss -tlnp | grep 8181

# 2. Kill the process
sudo kill $(sudo lsof -t -i:8181)

# 3. Or change GRAIN port in config.yml
server:
  port: ":8182"  # Use different port
```

### Permission Denied

**Problem**: `permission denied` errors

**Solutions**:

```bash
# 1. Check file ownership
ls -la grain

# 2. Make executable
chmod +x grain

# 3. Check directory permissions
ls -la www/

# 4. For service installation
sudo chown -R grain:grain /home/grain/grain-relay
```

### Configuration File Errors

**Problem**: `failed to load config` or YAML errors

**Solutions**:

```bash
# 1. Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config.yml'))"

# 2. Check for tabs vs spaces (use spaces only)
cat -A config.yml | head -20

# 3. Reset to defaults
rm config.yml
# Start GRAIN to regenerate default config
```

### Memory Issues

**Problem**: Out of memory or high memory usage

**Solutions**:

```yaml
# Reduce resource limits in config.yml
resource_limits:
  memory_mb: 512 # Lower memory limit
  heap_size_mb: 400 # Lower heap limit

# Enable aggressive purging
event_purge:
  enabled: true
  keep_interval_hours: 12 # Keep less data
```

### Network Connectivity

**Problem**: Can't connect externally

**Solutions**:

```bash
# 1. Check firewall
sudo ufw allow 8181
sudo iptables -A INPUT -p tcp --dport 8181 -j ACCEPT

# 2. Bind to all interfaces (config.yml)
server:
  port: "0.0.0.0:8181"  # Listen on all interfaces

# 3. Check reverse proxy configuration (nginx/apache)
```

### Working Directory Issues

**Problem**: `www directory not found` or similar errors

**Solutions**:

```bash
# 1. Ensure you're running GRAIN from the correct directory
cd /path/to/your/grain/directory
ls -la  # Should show grain executable and www/ directory

# 2. Check if www directory exists and has correct structure
ls -la www/
# Should show: static/, style/, views/

# 3. For service installations, verify WorkingDirectory is correct
sudo systemctl status grain
# Check WorkingDirectory in service file
```

---

## Next Steps

Your GRAIN relay is now installed and running! Here's what to do next:

### 1. Configuration

- **Customize settings**: Review and adjust [configuration options](../docs/configuration.md)
- **Set up moderation**: Configure whitelist/blacklist policies
- **Optimize performance**: Tune rate limits and resource usage

### 2. Security

- **Enable HTTPS**: Set up SSL/TLS certificates
- **Configure firewall**: Restrict access to necessary ports
- **Set up authentication**: Enable NIP-42 if needed
- **Regular updates**: Keep GRAIN and MongoDB updated

### 3. Monitoring

- **Set up logging**: Configure log rotation and analysis
- **Monitor resources**: Track CPU, memory, and disk usage
- **Database maintenance**: Regular MongoDB optimization
- **Backup strategy**: Implement data backup procedures

### 4. Community

- **Join discussions**: Connect with other GRAIN operators
- **Report issues**: Submit bugs and feature requests
- **Contribute**: Help improve GRAIN development

### 5. Advanced Features

- **Docker deployment**: Scale with containers
- **Load balancing**: Handle high traffic
- **Multi-relay setup**: Federate with other relays
- **Custom modifications**: Extend GRAIN functionality

### Resources

- **Configuration Guide**: [docs/configuration.md](../docs/configuration.md)
- **Development Guide**: [development/README.md](../development/README.md)
- **Docker Documentation**: [docker/README.md](../docker/README.md)
- **Testing Guide**: [tests/README.md](../tests/README.md)
- **GitHub Repository**: [https://github.com/0ceanslim/grain](https://github.com/0ceanslim/grain)
- **Issue Tracker**: [https://github.com/0ceanslim/grain/issues](https://github.com/0ceanslim/grain/issues)

---

**Installation Complete!** 🌾

Your GRAIN relay is ready to serve the Nostr network. Welcome to the decentralized web!
