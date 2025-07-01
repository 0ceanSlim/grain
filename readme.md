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

- Supports all Nostr event categories: regular posts, user profiles, replaceable events, and ephemeral messages
- Automatic event deletion handling (kind 5 events) with proper cascade cleanup
- Intelligent event purging with category-based retention policies
- MongoDB storage optimized for Nostr's event structure

### **Performance Focused**

- Unified cross-collection database queries for efficient event retrieval
- Per-kind MongoDB collections with automatic indexing
- Configurable event size limits to prevent abuse
- Connection pooling and timeout management

## Event Processing

GRAIN handles all Nostr event types according to protocol specifications:

- **Regular events** (kind 1 notes, kind 7 reactions) - stored permanently
- **Replaceable events** (kind 0 profiles, kind 3 contact lists) - newest version kept
- **Addressable events** (kind 30000+ with 'd' tags) - replaced by newer versions with same identifier
- **Ephemeral events** (kind 20000-30000) - processed but not stored
- **Deletion events** (kind 5) - removes referenced events if authored by same user

## Web Interface

GRAIN includes a basic web interface accessible at `http://your-relay-domain:port`:

- NIP-11 relay metadata served at the root with proper CORS headers for client discovery
- User login system that displays basic profile information for users who exist on the relay
- Simple API endpoints for checking lists and relay status
- Static file serving including favicon and basic assets

The frontend is currently minimal but functional. Future development will expand this into a reference Nostr client implementation with comprehensive relay metrics and management APIs.

## 🌾 Wheat Relay Status

![Status](https://img.shields.io/badge/Status-up-brightgreen)
![Uptime](https://img.shields.io/badge/Uptime%2024h-100.00%25-brightgreen)
![Response Time](https://img.shields.io/badge/Response%20Time-427ms-blue)

**Development Relay**: `wss://wheat.happytavern.co`

My development relay **wheat.happytavern.co** serves as the testing and demo environment for Grain. This relay helps me validate new features, test performance optimizations, and provide a stable platform for developers to experiment with grain.

Wheat is a public nostr relay that anyone can write to and read from. Wheat will delete events from non whitelisted users periodically. You can add your npub to the whitelist by paying for a [Happy Tavern NIP05](https://happytavern.co/nostr-verified).

_Status monitoring powered by GitHub Actions with 5-minute check intervals_

## Requirements

- **Go** if building from source
  - <img src="https://go.dev/images/favicon-gopher.svg" width="16"/> _[Download Go](https://go.dev/)_
- **MongoDB** for event storage and indexing
  - <img src="https://www.mongodb.com//assets/images/global/favicon.ico" width="20"/> _[MongoDB Community Server Install Docs](https://www.mongodb.com/docs/manual/administration/install-community/)_

## Installation

### Using Pre-built Binaries (Recommended)

1. **Download the latest release** for your system from the releases page
2. **Extract the archive** and ensure both the binary and `www` folder are in the same directory:

   grain/  
   ├── grain (or grain.exe on Windows)  
   └── www/

**Start MongoDB** - GRAIN requires a running MongoDB instance (default: `localhost:27017`)

**Run GRAIN** - `./grain` (Linux) or `grain.exe` (Windows)

GRAIN will automatically create default configuration files on first run and start serving on port `:8181`.

Edit config files and GRAIN automatically restarts with new settings

### Building from Source

If pre-built binaries aren't available for your architecture you can clone this repo and build the binary from source:

```bash
git clone https://github.com/0ceanslim/grain.git
cd grain
go build -o grain .
./grain
```

## Configuration

GRAIN uses four main configuration files with hot-reload support:

- `config.yml` - Server, database, and rate limiting settings
- `whitelist.yml` - Allowed users, domains, and event types
- `blacklist.yml` - Banned content and escalation policies
- `relay_metadata.json` - Public relay information (NIP-11)

For detailed configuration options and examples, see:

[**Example configurations**](https://github.com/0ceanslim/grain/tree/main/www/static/examples)

### Monitoring and Logs

GRAIN provides detailed operational visibility:

```yaml
logging:
  level: "info" # Log levels: "debug", "info", "warn", "error"
  file: "debug" # Log file name
  max_log_size_mb: 10 # Maximum log file size in MB before trimming
  structure: false # true = structured JSON logs, false = pretty logs
  check_interval_min: 10 # Check every 10 minutes
  backup_count: 2 # Keep 2 backup files (.bak1, .bak2)
  suppress_components: # Components to suppress INFO/DEBUG logs from (WARN/ERROR still shown)
    - "util" # Utility functions (file ops, IP detection, metadata loading)
    - "conn-manager" # Connection management (memory stats, connection counts)
    - "client" # Client connection details (connects/disconnects, timeouts)
    - "mongo-query" # Database query operations (can be very verbose)
    - "event-store" # Event storage operations (insert/update confirmations)
    - "close-handler" # Subscription close operations (routine cleanup)

# Available components for suppression:
# - "main"             # Main application lifecycle (startup, shutdown, restarts)
# - "mongo"            # MongoDB connection and database operations
# - "mongo-store"      # High-level event storage coordination
# - "mongo-purge"      # Event purging and cleanup operations
# - "event-handler"    # Event processing and validation coordination
# - "req-handler"      # Subscription request handling
# - "auth-handler"     # Authentication processing (NIP-42)
# - "config"           # Configuration loading and caching
# - "event-validation" # Event signature and content validation
# - "user-sync"        # User synchronization operations
# - "log"              # Logging system internal operations

# Note: Suppression only affects INFO and DEBUG levels.
# WARN and ERROR messages are always shown regardless of suppression.
```

Built-in metrics include:

- Active WebSocket connections and memory usage
- Event processing rates and error counts
- Database query performance
- Cache hit rates for whitelist/blacklist operations

### Authentication

Optional user authentication via NIP-42:

```yaml
auth:
  enabled: false
  relay_url: "wss://your-relay.com"
```

When enabled, clients must authenticate before publishing events or accessing restricted content.

### Automatic Event Purging

Keep your database clean with configurable retention:

```yaml
event_purge:
  enabled: true
  keep_interval_hours: 720 # 30 days
  purge_interval_minutes: 60 # check hourly
  exclude_whitelisted: true # never purge whitelisted users
  purge_by_category:
    regular: true # purge old posts and reactions
    ephemeral: true # purge ephemeral events (shouldn't be stored anyway)
    replaceable: false # keep user profiles and contact lists
```

### User Synchronization (Experimental)

⚠️ **Work in Progress**: User sync is an experimental feature that may contain bugs and is subject to change.

GRAIN can attempt to automatically sync new users' event history from their preferred relays:

```yaml
UserSync: # EXPERIMENTAL FEATURE, (structured logging not implemented yet)
  user_sync: false # disabled by default
  disable_at_startup: true
  initial_sync_relays: [
      "wss://purplepag.es",
      "wss://nos.lol",
      "wss://relay.damus.io",
    ] # These relays are used to initially fetch user Outboxes.
  kinds: [1, 0, 7] # sync posts, profiles, reactions. If kinds is left empty, no kind is applied to the filter and any event is retrieved
  limit: 100 # If limit is left empty, no limit will be applied to the filter
  exclude_non_whitelisted: true # if set to true, only pubkeys on the whitelist will be synced.
  interval: 360 # in minutes
```

When enabled and a new user posts to your relay, GRAIN attempts to:

1. Query configured relays for the user's relay metadata (kind 10002)
2. Fetch their recent events from their preferred "outbox" relays
3. Store missing events locally

**Known limitations**: This feature is experimental and may cause performance issues or sync failures. Use with caution in production environments.

## License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.

## Contributing

I welcome contributions, bug reports, and feature requests via GitHub.

**Repository**: <https://github.com/0ceanslim/grain>  
**Issues**: <https://github.com/0ceanslim/grain/issues>

---

made with 💦 by [OceanSlim](https://njump.me/npub1zmc6qyqdfnllhnzzxr5wpepfpnzcf8q6m3jdveflmgruqvd3qa9sjv7f60)

_Reliable infrastructure for the decentralized web._
