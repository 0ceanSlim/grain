# GRAIN 🌾

## Go Relay Architecture for Implementing Nostr

GRAIN is a nostr relay designed for operators who need fine-grained control over their relay's behavior.

## What is Nostr?

Nostr is a simple, open protocol for creating censorship-resistant social networks. Users publish signed events (posts, profiles, reactions) to relays, which store and distribute them. Unlike centralized platforms, users control their identity through cryptographic keys and can freely move between relays.

GRAIN acts as one of these relays - storing events, serving them to clients, and ensuring your relay operates according to your policies.

## Why GRAIN?

### **Intelligent Content Control**

- Real-time blacklist/whitelist filtering with automatic caching
- Word-based content filtering that escalates temporary bans to permanent ones
- Import blacklists from Nostr mute lists (kind 10000 events)
- Domain-based whitelisting by fetching pubkeys from `.well-known/nostr.json`

### **Intelligent Management**

- Hot configuration reloading - change settings without restarting
- Comprehensive structured logging with automatic rotation
- Memory-aware connection management prevents resource exhaustion
- Multi-layer rate limiting (connections, events, queries) with per-kind controls

### **Event Management**

- Supports all Nostr event categories: regular, replaceable, addressable, ephemeral & deletion events
- Automatic event deletion handling (kind 5 events) with proper cascade cleanup
- Intelligent event purging with category-based retention policies
- MongoDB storage optimized for Nostr's event structure

### **Performance Focused**

- Unified cross-collection database queries for efficient event retrieval
- Per-kind MongoDB collections with automatic indexing
- Configurable event size limits to prevent abuse
- Connection pooling and timeout management

## Web Interface

GRAIN includes a basic web interface accessible at `http://your-relay-domain:port`:

- NIP-11 relay metadata served at the root with proper CORS headers for client discovery
- User login system that displays basic profile information for users who exist on the relay
- Simple API endpoints for checking lists and relay status
- Static file serving including favicon and basic assets

The frontend is currently minimal but functional. Future development will expand this into a reference Nostr client implementation with comprehensive relay metrics and management APIs.

## 🌾 Wheat Relay Status

[![Status](https://img.shields.io/endpoint?url=https://0ceanslim.github.io/grain/api/status-badge.json)](https://0ceanslim.github.io/grain/)
[![Uptime 24h](https://img.shields.io/endpoint?url=https://0ceanslim.github.io/grain/api/24h-uptime-badge.json)](https://0ceanslim.github.io/grain/)
[![Uptime 90d](https://img.shields.io/endpoint?url=https://0ceanslim.github.io/grain/api/90d-uptime-badge.json)](https://0ceanslim.github.io/grain/)

**Development Relay**: `wss://wheat.happytavern.co`

[📊 **View Detailed Status & Historical Data**](https://0ceanslim.github.io/grain/)

My development relay **wheat.happytavern.co** serves as the testing and demo environment for Grain. This relay helps me validate new features, test performance optimizations, and provide a platform for developers to experiment with grain. This relay routinely runs unreleased versions of grain and may contain bugs.

Wheat is a public nostr relay that anyone can write to and read from. Wheat will delete events from non whitelisted users periodically. You can add your npub to the whitelist by paying for a [Happy Tavern NIP05](https://happytavern.co/nostr-verified).

_Status monitoring powered by GitHub Actions with 5-minute check intervals_

## Installation

### Quick Start (Recommended)

1. **Download the latest release** for your system from the [releases page](https://github.com/0ceanslim/grain/releases)
2. **Extract the archive** and ensure both the binary and `www` folder are in the same directory:

   ```
   grain/
   ├── grain (or grain.exe on Windows)
   └── www/
   ```

3. **Install MongoDB** for your system:

   - <img src="https://www.mongodb.com//assets/images/global/favicon.ico" width="20"/> _[MongoDB Community Server Install Guide](https://www.mongodb.com/docs/manual/administration/install-community/)_

4. **Run GRAIN** - `./grain` (Linux) or `grain.exe` (Windows)

GRAIN will automatically create default configuration files on first run and start serving on port `:8181`.

### Alternative Installation Methods

**Docker Installation**

- <img src="https://www.docker.com/app/uploads/2024/02/cropped-docker-logo-favicon-32x32.png" width="20"/> _[Docker Setup Guide](docs/docker/readme.md)_ (includes MongoDB)

**Build from Source**

- **Requirements**: <img src="https://go.dev/images/favicon-gopher.svg" width="16"/> _[Go 1.21+](https://go.dev/)_ + MongoDB

**Complete Installation Guide**

- _[Full Installation Documentation](docs/installation.md)_ - includes system service setup for Linux (systemd) and Windows (NSSM)

> **Note**: Docker installations include MongoDB automatically. For other methods, you'll need to install MongoDB separately.

## Configuration

GRAIN uses four main configuration files with hot-reload support:

- `config.yml` - Server, database, and rate limiting settings
- `whitelist.yml` - Allowed users, domains, and event types
- `blacklist.yml` - Banned content and escalation policies
- `relay_metadata.json` - Public relay information (NIP-11)

Configuration files are automatically created from embedded examples on first run. Changes to any configuration file are detected and applied automatically without requiring a restart.

**📖 For detailed configuration options and examples, see [Configuration Documentation](docs/configuration.md)**

## License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.

## Contributing

I welcome contributions, bug reports, and feature requests via GitHub.

**Repository**: <https://github.com/0ceanslim/grain>  
**Issues**: <https://github.com/0ceanslim/grain/issues>

### Development Resources

- **🔧 Development Guide** - _[Development Documentation](docs/development/)_
- **🧪 Testing Guide** - _[Testing Documentation](tests/readme.md)_

## These guides cover setting up your development environment, code standards, testing procedures, and contribution workflows.

made with 💦 by [OceanSlim](https://njump.me/npub1zmc6qyqdfnllhnzzxr5wpepfpnzcf8q6m3jdveflmgruqvd3qa9sjv7f60)

_Reliable infrastructure for the decentralized web._
