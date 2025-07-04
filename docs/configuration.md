# GRAIN Configuration Guide

Comprehensive documentation for configuring your GRAIN relay server.

## Table of Contents

- [GRAIN Configuration Guide](#grain-configuration-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
    - [File Hierarchy](#file-hierarchy)
  - [Configuration Files](#configuration-files)
    - [File Creation](#file-creation)
  - [Server Configuration (`config.yml`)](#server-configuration-configyml)
    - [Logging Configuration](#logging-configuration)
      - [Log Levels](#log-levels)
      - [Structured vs Pretty Logging](#structured-vs-pretty-logging)
      - [Component Suppression](#component-suppression)
    - [MongoDB Configuration](#mongodb-configuration)
      - [Connection String Options](#connection-string-options)
    - [Server Settings](#server-settings)
      - [Timeout Configuration](#timeout-configuration)
      - [Subscription Management](#subscription-management)
    - [Resource Limits](#resource-limits)
      - [CPU Management](#cpu-management)
      - [Memory Management](#memory-management)
    - [Authentication (NIP-42)](#authentication-nip-42)
      - [Authentication Flow](#authentication-flow)
      - [Use Cases](#use-cases)
    - [User Synchronization (Experimental)](#user-synchronization-experimental)
      - [Sync Process](#sync-process)
      - [Performance Impact](#performance-impact)
    - [Backup Relay](#backup-relay)
      - [Backup Strategy](#backup-strategy)
    - [Event Purging](#event-purging)
      - [Purge Categories](#purge-categories)
    - [Event Time Constraints](#event-time-constraints)
      - [Time Validation Options](#time-validation-options)
      - [Time Format Examples](#time-format-examples)
    - [Rate Limiting](#rate-limiting)
      - [Rate Limiting Layers](#rate-limiting-layers)
      - [Category-Specific Rate Limits](#category-specific-rate-limits)
      - [Kind-Specific Rate Limits](#kind-specific-rate-limits)
    - [Size Limiting](#size-limiting)
      - [Size Limit Guidelines](#size-limit-guidelines)
  - [Whitelist Configuration (`whitelist.yml`)](#whitelist-configuration-whitelistyml)
    - [Pubkey Whitelist](#pubkey-whitelist)
      - [Whitelist Behavior](#whitelist-behavior)
    - [Kind Whitelist](#kind-whitelist)
      - [Common Kind Combinations](#common-kind-combinations)
    - [Domain Whitelist](#domain-whitelist)
      - [Domain Verification Process](#domain-verification-process)
      - [Domain Whitelist Use Cases](#domain-whitelist-use-cases)
  - [Blacklist Configuration (`blacklist.yml`)](#blacklist-configuration-blacklistyml)
    - [Content Filtering](#content-filtering)
      - [Ban Escalation System](#ban-escalation-system)
      - [Word Filtering Strategy](#word-filtering-strategy)
    - [Permanent Blacklist](#permanent-blacklist)
    - [Mute List Integration](#mute-list-integration)
      - [Mute List Process](#mute-list-process)
      - [Mute List Benefits](#mute-list-benefits)
  - [Relay Metadata (`relay_metadata.json`)](#relay-metadata-relay_metadatajson)
  - [Configuration Validation](#configuration-validation)
    - [Validation Rules](#validation-rules)
    - [Error Handling](#error-handling)
  - [Performance Tuning](#performance-tuning)
    - [High-Traffic Relays](#high-traffic-relays)
    - [Low-Resource Deployments](#low-resource-deployments)
    - [Private/Curated Relays](#privatecurated-relays)
  - [Security Considerations](#security-considerations)
    - [Access Control](#access-control)
    - [Content Moderation](#content-moderation)
    - [Data Protection](#data-protection)
  - [Troubleshooting](#troubleshooting)
    - [Database Connection Issues](#database-connection-issues)
    - [Rate Limiting Too Strict](#rate-limiting-too-strict)
    - [Configuration Reload Failures](#configuration-reload-failures)

---

## Overview

GRAIN uses a multi-file configuration system that supports hot-reload. This allows you to adjust relay behavior without restarting the server, making it ideal for production deployments.

### File Hierarchy

```
grain/
├── config.yml              # Main server configuration
├── whitelist.yml           # User and content allowlists
├── blacklist.yml           # User and content blocklists
└── relay_metadata.json     # Public relay information (NIP-11)
```

---

## Configuration Files

### File Creation

GRAIN automatically creates default configuration files on first run:

- **config.yml** - Created from `docs/examples/config.example.yml`
- **whitelist.yml** - Created from `docs/examples/whitelist.example.yml`
- **blacklist.yml** - Created from `docs/examples/blacklist.example.yml`
- **relay_metadata.json** - Created from `docs/examples/relay_metadata.example.json`

These are created from the example configs in the docs which are embedded into the binary now, so if the configs don't exist and grain tries to run it will create the example configs that are missing using the embedded example configs from the docs directory.

---

## Server Configuration (`config.yml`)

The primary configuration file containing all server-level settings.

### Logging Configuration

Controls log output, rotation, and component filtering.

```yaml
logging:
  level: "info" # Log level: debug, info, warn, error
  file: "debug" # Log file base name
  max_log_size_mb: 10 # Maximum log file size before rotation
  structure: false # true = JSON logs, false = pretty logs
  check_interval_min: 10 # Log size check frequency
  backup_count: 2 # Number of backup files to keep
  suppress_components: # Component log suppression (INFO/DEBUG only)
    - "util" # Utility operations
    - "mongo-query" # Database query operations
    - "mongo-store" # Event storage operations
    - "relay-client" # Relay client connections
    - "close-handler" # Subscription close operations
```

#### Log Levels

| Level   | Description              | Use Case                     |
| ------- | ------------------------ | ---------------------------- |
| `debug` | Verbose debugging info   | Development, troubleshooting |
| `info`  | General operational info | Production monitoring        |
| `warn`  | Warning conditions       | Issue detection              |
| `error` | Error conditions only    | Minimal logging              |

#### Structured vs Pretty Logging

**Structured Logging (`structure: true`)**

- Output: JSON format for machine parsing
- File: `debug.log.json`
- Benefits: Searchable, parseable by log aggregators
- Use case: Production environments with log analysis tools

**Pretty Logging (`structure: false`)**

- Output: Human-readable format
- File: `debug.log`
- Benefits: Easy to read, better for development
- Use case: Development and simple deployments

#### Component Suppression

Available components for `suppress_components`:

| Component             | Purpose                       | Recommended for Suppression |
| --------------------- | ----------------------------- | --------------------------- |
| **Core Components**   |                               |                             |
| `startup`             | Startup operations            | ❌ Keep for debugging       |
| `config`              | Configuration loading         | ❌ Keep for hot-reload info |
| `util`                | Utility functions             | ✅ Low importance           |
| `log`                 | Logging system operations     | ✅ Meta-logging noise       |
| **Database**          |                               |                             |
| `mongo`               | MongoDB connection            | ❌ Keep for debugging       |
| `mongo-query`         | Query operations              | ✅ Can be very verbose      |
| `mongo-store`         | Storage operations            | ✅ High frequency           |
| `mongo-purge`         | Event purging                 | ❌ Keep for maintenance     |
| **Event Processing**  |                               |                             |
| `event-handler`       | Event processing coordination | ❌ Keep for monitoring      |
| `event-validation`    | Event signature validation    | ❌ Keep for security        |
| `event-store`         | Event storage operations      | ✅ High frequency           |
| **Message Handlers**  |                               |                             |
| `req-handler`         | REQ subscription handling     | ❌ Keep for monitoring      |
| `auth-handler`        | AUTH message handling         | ❌ Keep for security        |
| `close-handler`       | CLOSE subscription handling   | ✅ Routine operations       |
| **Relay Operations**  |                               |                             |
| `relay-client`        | Relay client connections      | ✅ Very verbose             |
| `relay-connection`    | Relay connection management   | ✅ Can be verbose           |
| `relay-api`           | Relay API operations          | ❌ Keep for API monitoring  |
| **Client Components** |                               |                             |
| `client-main`         | Client main operations        | ✅ Can be verbose           |
| `client-api`          | Client API operations         | ✅ Can be verbose           |
| `client-core`         | Client core functionality     | ✅ Can be verbose           |
| `client-tools`        | Client utility tools          | ✅ Low importance           |
| `client-data`         | Client data operations        | ✅ Can be verbose           |
| `client-connection`   | Client connection management  | ✅ Can be verbose           |
| `client-session`      | Client session management     | ✅ Can be verbose           |
| `client-cache`        | Client caching operations     | ✅ Can be verbose           |
| **Other Components**  |                               |                             |
| `user-sync`           | User synchronization          | ❌ Keep for sync monitoring |

**Note**: Suppression only affects INFO and DEBUG log levels. WARN and ERROR messages are always shown regardless of suppression settings.

### MongoDB Configuration

Database connection and settings.

```yaml
mongodb:
  uri: "mongodb://localhost:27017/" # MongoDB connection string
  database: "grain" # Database name
```

#### Connection String Options

```yaml
# Basic local connection
uri: "mongodb://localhost:27017/"

# Authenticated connection
uri: "mongodb://username:password@localhost:27017/"

# Replica set connection
uri: "mongodb://host1:27017,host2:27017/grain?replicaSet=rs0"

# MongoDB Atlas connection
uri: "mongodb+srv://user:pass@cluster.mongodb.net/"

# Connection with options
uri: "mongodb://localhost:27017/?maxPoolSize=20&retryWrites=true"
```

### Server Settings

Core server behavior and connection handling.

```yaml
server:
  port: ":8181" # Listen port
  read_timeout: 60 # WebSocket read timeout (seconds)
  write_timeout: 20 # WebSocket write timeout (seconds)
  idle_timeout: 1200 # Connection idle timeout (seconds)
  max_subscriptions_per_client: 10 # Maximum concurrent subscriptions
  implicit_req_limit: 500 # Default REQ limit when none specified
```

#### Timeout Configuration

**Read Timeout (`read_timeout`)**

- Purpose: Maximum time to wait for a message from client
- Default: 60 seconds
- Tuning: Lower for faster client detection, higher for slow connections

**Write Timeout (`write_timeout`)**

- Purpose: Maximum time to send a message to client
- Default: 20 seconds
- Tuning: Lower for network congestion detection, higher for slow clients

**Idle Timeout (`idle_timeout`)**

- Purpose: Close connections with no activity
- Default: 1200 seconds (20 minutes)
- Special: Set to 0 to disable idle timeout completely

#### Subscription Management

**Max Subscriptions Per Client (`max_subscriptions_per_client`)**

- Purpose: Prevent resource exhaustion from too many subscriptions
- Default: 10
- Behavior: Oldest subscription dropped when limit exceeded

**Implicit REQ Limit (`implicit_req_limit`)**

- Purpose: Default limit applied when client doesn't specify one
- Default: 500
- Impact: Affects initial query response size

### Resource Limits

System resource constraints and memory management.

```yaml
resource_limits:
  cpu_cores: 2 # CPU core limit
  memory_mb: 1024 # RAM limit (MB)
  heap_size_mb: 512 # Go heap limit (MB)
```

#### CPU Management

**CPU Cores (`cpu_cores`)**

- Purpose: Limit CPU usage via `GOMAXPROCS`
- Default: System CPU count
- Tuning: Set based on available hardware and other services

#### Memory Management

**Memory Limit (`memory_mb`)**

- Purpose: Overall memory monitoring and alerting
- Behavior: Triggers garbage collection when exceeded
- Note: This is monitoring, not enforcement

**Heap Size Limit (`heap_size_mb`)**

- Purpose: Go garbage collector heap limit
- Behavior: Forces GC when heap approaches limit
- Tuning: Set to 60-80% of available memory

### Authentication (NIP-42)

Optional client authentication using Nostr's NIP-42 standard.

```yaml
auth:
  enabled: false # Enable/disable authentication
  relay_url: "wss://relay.example.com/" # Relay URL for challenge
```

#### Authentication Flow

When enabled, clients must authenticate before publishing events:

1. Client connects to relay
2. Relay sends `AUTH` challenge
3. Client signs authentication event (kind 22242)
4. Relay verifies signature and challenge
5. Client is authenticated for session duration

#### Use Cases

- **Private relays** - Restrict access to known users
- **Paid relays** - Verify subscription status
- **Moderated communities** - Control posting privileges

### User Synchronization (Experimental)

**⚠️ Experimental Feature** - May contain bugs and performance issues.

```yaml
UserSync:
  user_sync: false # Enable/disable user sync
  disable_at_startup: true # Skip sync on startup
  initial_sync_relays: # Relays for outbox discovery
    - "wss://purplepag.es"
    - "wss://nos.lol"
    - "wss://relay.damus.io"
  kinds: [1, 0, 7] # Event kinds to sync
  limit: 100 # Events per sync operation
  exclude_non_whitelisted: true # Only sync whitelisted users
  interval: 360 # Sync interval (minutes)
```

#### Sync Process

1. User posts to your relay for the first time
2. GRAIN queries `initial_sync_relays` for user's relay list (kind 10002)
3. Fetches recent events from user's preferred "outbox" relays
4. Stores missing events locally

#### Performance Impact

- **Network intensive** - Multiple relay connections per user
- **Storage impact** - Significant database growth
- **CPU usage** - Event processing and validation overhead

### Backup Relay

Forward events to a backup relay for redundancy.

```yaml
backup_relay:
  enabled: false # Enable backup forwarding
  url: "wss://backup-relay.com" # Backup relay WebSocket URL
```

#### Backup Strategy

- **Asynchronous** - Doesn't block main event processing
- **Best effort** - Failures don't affect primary operation
- **Event forwarding** - All successfully stored events are forwarded

### Event Purging

Automatic cleanup of old events to manage database size.

```yaml
event_purge:
  enabled: false # Enable automatic purging
  disable_at_startup: true # Skip purge on startup
  keep_interval_hours: 24 # Hours to keep events
  purge_interval_minutes: 240 # Purge check frequency
  purge_by_category: # Category-specific rules
    regular: true # Purge kind 1, 4-44, 1000-9999
    replaceable: false # Keep kind 0, 3, 10000-19999
    addressable: false # Keep kind 30000-39999
    deprecated: true # Purge deprecated kinds
  purge_by_kind_enabled: false # Enable kind-specific purging
  kinds_to_purge: [1, 2, 1000] # Specific kinds to purge
  exclude_whitelisted: true # Never purge whitelisted users
```

#### Purge Categories

**Regular Events**

- Kinds: 1, 4-44, 1000-9999
- Content: Posts, DMs, reactions, most social content
- Recommendation: Enable purging for storage management

**Replaceable Events**

- Kinds: 0, 3, 10000-19999
- Content: Profiles, contact lists, metadata
- Recommendation: Keep (only newest version stored anyway)

**Addressable Events**

- Kinds: 30000-39999
- Content: Long-form posts, lists, custom content
- Recommendation: Keep (often important content)

**Deprecated Events**

- Kinds: 2 and other deprecated event types
- Recommendation: Enable purging

### Event Time Constraints

Validation of event timestamps to prevent abuse.

```yaml
event_time_constraints:
  min_created_at: 1577836800 # Minimum timestamp (Unix)
  min_created_at_string: "now-5m" # Relative minimum time
  max_created_at: 0 # Maximum timestamp (0 = now)
  max_created_at_string: "now+5m" # Relative maximum time
```

#### Time Validation Options

**Absolute Timestamps**

```yaml
min_created_at: 1577836800 # January 1, 2020
max_created_at: 1735689600 # January 1, 2025
```

**Relative Timestamps**

```yaml
min_created_at_string: "now-24h" # 24 hours ago
max_created_at_string: "now+1h" # 1 hour in future
```

#### Time Format Examples

| Format    | Description      | Example                  |
| --------- | ---------------- | ------------------------ |
| `now-5m`  | 5 minutes ago    | Current time - 5 minutes |
| `now+1h`  | 1 hour in future | Current time + 1 hour    |
| `now-24h` | 24 hours ago     | Current time - 24 hours  |
| `now-7d`  | 7 days ago       | Current time - 7 days    |

### Rate Limiting

Multi-layer rate limiting to prevent abuse and ensure fair usage.

```yaml
rate_limit:
  ws_limit: 50 # WebSocket messages/second
  ws_burst: 100 # WebSocket burst allowance
  event_limit: 10 # Events/second
  event_burst: 20 # Event burst allowance
  req_limit: 5 # REQ queries/second
  req_burst: 15 # REQ burst allowance
  max_event_size: 524288 # Maximum event size (bytes)
```

#### Rate Limiting Layers

**WebSocket Layer (`ws_limit`, `ws_burst`)**

- Scope: All WebSocket messages (EVENT, REQ, CLOSE)
- Purpose: Prevent message flooding
- Tuning: High limits for active relays, lower for private relays

**Event Layer (`event_limit`, `event_burst`)**

- Scope: EVENT messages only
- Purpose: Prevent event spam
- Tuning: Based on expected publishing rate

**REQ Layer (`req_limit`, `req_burst`)**

- Scope: REQ (subscription) messages
- Purpose: Prevent query abuse
- Tuning: Lower limits (queries are expensive)

#### Category-Specific Rate Limits

```yaml
rate_limit:
  category_limits:
    regular: # Kind 1, 4-44, 1000-9999
      limit: 8 # Events/second
      burst: 16 # Burst allowance
    replaceable: # Kind 0, 3, 10000-19999
      limit: 2 # Events/second
      burst: 5 # Burst allowance
    ephemeral: # Kind 20000-29999
      limit: 50 # Events/second
      burst: 100 # Burst allowance
    addressable: # Kind 30000-39999
      limit: 3 # Events/second
      burst: 8 # Burst allowance
```

#### Kind-Specific Rate Limits

```yaml
rate_limit:
  kind_limits:
    - kind: 0 # Profile updates
      limit: 1 # Events/second
      burst: 2 # Burst allowance
    - kind: 1 # Text notes
      limit: 5 # Events/second
      burst: 12 # Burst allowance
```

> **Note**:this can be used to effectively blacklist by event kind by setting the rate limit to 0

### Size Limiting

Control event sizes to prevent abuse and manage memory usage.

```yaml
rate_limit:
  max_event_size: 524288 # Global size limit (512KB)
  kind_size_limits:
    - kind: 0 # User metadata
      max_size: 8192 # 8KB limit
    - kind: 1 # Text notes
      max_size: 4096 # 4KB limit
    - kind: 3 # Follow lists
      max_size: 65536 # 64KB limit
    - kind: 7 # Reactions
      max_size: 512 # 512B limit
```

#### Size Limit Guidelines

| Event Kind             | Typical Size | Recommended Limit | Purpose                            |
| ---------------------- | ------------ | ----------------- | ---------------------------------- |
| Kind 0 (Profile)       | 1-2KB        | 8KB               | Allow rich profiles with images    |
| Kind 1 (Text Note)     | 100-500B     | 4KB               | Prevent spam, allow normal posts   |
| Kind 3 (Follow List)   | 10-20KB      | 64KB              | Support large follow lists (1000+) |
| Kind 7 (Reaction)      | 50-100B      | 512B              | Reactions should be minimal        |
| Kind 30023 (Long-form) | 5-50KB       | 100KB             | Allow substantial articles         |

---

## Whitelist Configuration (`whitelist.yml`)

Control which users, content types, and domains are allowed.

### Pubkey Whitelist

Allow specific users by their public keys or npubs.

```yaml
pubkey_whitelist:
  enabled: false # Enable pubkey filtering
  pubkeys: # Hex public keys
    - "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
    - "fa984bd7dbb282f07e16e7ae87b26a2a7b9b90b7246a44771f0cf5ae58018f52"
  npubs: # Bech32 encoded public keys
    - "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
    - "npub1l2vgasny2md9sqha5fdha44269hss6xqn3mrhzp34l5dtu0cqq4sl4jvr0"
  cache_refresh_minutes: 60 # Cache refresh frequency
```

#### Whitelist Behavior

**When Enabled (`enabled: true`)**

- Only specified pubkeys can publish events
- All other pubkeys are rejected
- Useful for private or curated relays

**When Disabled (`enabled: false`)**

- All pubkeys are allowed (unless blacklisted)
- Whitelist is cached but not enforced
- Used for sync and purge operations

### Kind Whitelist

Allow only specific event types.

```yaml
kind_whitelist:
  enabled: false # Enable kind filtering
  kinds: # Allowed event kinds
    - "0" # User metadata
    - "1" # Text notes
    - "3" # Contact lists
    - "7" # Reactions
    - "10002" # Relay lists
```

#### Common Kind Combinations

**Social Media Relay**

```yaml
kinds: ["0", "1", "3", "7", "9735"] # Profiles, posts, follows, reactions, zaps
```

**Metadata Only**

```yaml
kinds: ["0", "3", "10002"] # Profiles, contacts, relay lists
```

**Full Protocol Support**

```yaml
kinds: [] # Empty = allow all kinds
```

### Domain Whitelist

Allow users who have verified at a domain via NIP-05.

```yaml
domain_whitelist:
  enabled: false # Enable domain filtering
  domains: # Allowed domains
    - "example.com"
    - "nostr.example.org"
    - "verified-users.net"
  cache_refresh_minutes: 120 # Cache refresh frequency
```

#### Domain Verification Process

1. GRAIN fetches `https://domain.com/.well-known/nostr.json`
2. Extracts pubkeys from the `names` object
3. Allows events from those pubkeys
4. Refreshes cache periodically

#### Domain Whitelist Use Cases

- **Corporate relays** - Only company domain holders
- **Verified communities** - Users with established identities
- **Quality control** - Filter for serious users with domains

---

## Blacklist Configuration (`blacklist.yml`)

Block users and content based on various criteria.

### Content Filtering

Block events containing specific words or phrases.

```yaml
enabled: true # Enable blacklist system
permanent_ban_words: # Words triggering permanent bans
  - "spam-word"
  - "abusive-content"
temp_ban_words: # Words triggering temporary bans
  - "crypto"
  - "airdrop"
  - "web3"
max_temp_bans: 3 # Temp bans before permanent ban
temp_ban_duration: 3600 # Temp ban duration (seconds)
```

#### Ban Escalation System

1. **On offense** - Temporary ban (duration: `temp_ban_duration`)
2. **Subsequent offenses** - Temporary ban count incremented +
3. **Max violations** - Permanent ban after `max_temp_bans` violations

#### Word Filtering Strategy

**Permanent Ban Words**

- Clearly abusive content
- Illegal content
- Severe policy violations

**Temporary Ban Words**

- Spam-prone content
- Off-topic content
- Mild policy violations

### Permanent Blacklist

Explicitly banned users and npubs.

```yaml
permanent_blacklist_pubkeys: # Hex public keys
  - "abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"
permanent_blacklist_npubs: # Bech32 encoded keys
  - "npub1abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd"
```

### Mute List Integration

Import blacklists from Nostr mute list events (kind 10000).

```yaml
mutelist_authors: # Pubkeys of mute list authors
  - "3fe0ab6cbdb7ee27148202249e3fb3b89423c6f6cda6ef43ea5057c3d93088e4"
mutelist_cache_refresh_minutes: 30 # Refresh frequency
```

#### Mute List Process

1. GRAIN queries local relay for kind 10000 events from specified authors
2. Extracts `p` tags (muted pubkeys) from these events
3. Adds extracted pubkeys to blacklist cache
4. Refreshes periodically to stay current

#### Mute List Benefits

- **Community moderation** - Leverage trusted moderators' mute lists
- **Distributed blocking** - Share block lists across relays
- **Reduced maintenance** - Automatic updates from trusted sources

---

## Relay Metadata (`relay_metadata.json`)

GRAIN provides a NIP-11 compliant template constructed from default example configuration values by default.

For detailed NIP-11 configuration options and field specifications, refer to [NIP-11 documentation](https://github.com/nostr-protocol/nips/blob/master/11.md) and adjust the configuration to your relay's specific requirements.

---

## Configuration Validation

Validate all configuration changes before applying them.

### Validation Rules

**YAML Syntax**

- Valid YAML structure
- Proper indentation and formatting
- No duplicate keys

**Value Validation**

- Numeric ranges (e.g., timeouts > 0)
- Valid URLs and network addresses
- Proper hex keys and npub formats

**Logical Consistency**

- Rate limits less than burst limits
- Time constraints make sense
- File paths are accessible

### Error Handling

**Invalid Configuration**

```
2024-01-15 10:30:15 [ERROR] [config] Invalid configuration detected
  file=config.yml error="rate_limit.ws_limit must be positive"
  action="keeping previous configuration"
```

**Successful Reload**

```
2024-01-15 10:30:15 [INFO] [config] Configuration reloaded successfully
  file=config.yml changes=["logging.level", "rate_limit.event_limit"]
```

---

## Performance Tuning

Optimize GRAIN for your specific use case and hardware.

### High-Traffic Relays

**Increased Limits**

```yaml
server:
  max_subscriptions_per_client: 50
  implicit_req_limit: 1000

rate_limit:
  ws_limit: 200
  ws_burst: 500
  event_limit: 50
  event_burst: 100

resource_limits:
  cpu_cores: 8
  memory_mb: 4096
  heap_size_mb: 3200
```

**Aggressive Caching**

```yaml
pubkey_whitelist:
  cache_refresh_minutes: 30 # Less frequent refresh

domain_whitelist:
  cache_refresh_minutes: 60 # Domain verification caching
```

### Low-Resource Deployments

**Conservative Limits**

```yaml
server:
  max_subscriptions_per_client: 5
  implicit_req_limit: 100

rate_limit:
  ws_limit: 10
  ws_burst: 20
  event_limit: 2
  event_burst: 5

resource_limits:
  cpu_cores: 1
  memory_mb: 512
  heap_size_mb: 400
```

**Aggressive Purging**

```yaml
event_purge:
  enabled: true
  keep_interval_hours: 12 # Keep only 12 hours
  purge_interval_minutes: 60 # Hourly purging
```

### Private/Curated Relays

**Strict Access Control**

```yaml
# whitelist.yml
pubkey_whitelist:
  enabled: true # Only whitelisted users

# config.yml
auth:
  enabled: true # Require authentication

rate_limit:
  ws_limit: 100 # Higher limits for trusted users
  event_limit: 20
```

---

## Security Considerations

Configuration best practices for secure relay operation.

### Access Control

**Authentication**

- Enable NIP-42 authentication for private relays
- Use whitelist mode for curated communities
- Implement domain verification for verified users

**Network Security**

- Use HTTPS/WSS in production
- Configure proper CORS headers
- Implement rate limiting to prevent abuse

### Content Moderation

**Blacklist Strategy**

- Start with common spam words in `temp_ban_words`
- Add severe violations to `permanent_ban_words`
- Use mute list integration for community moderation

**Size Limits**

- Set reasonable event size limits
- Prevent large event spam
- Configure per-kind limits for different content types

### Data Protection

**Event Purging**

- Configure appropriate retention periods
- Exclude important user data from purging
- Consider legal requirements for data retention

**Backup Strategy**

- Use backup relay for redundancy
- Regular database backups
- Configuration file versioning

---

## Troubleshooting

Common configuration issues and solutions.

### Database Connection Issues

**Problem**: MongoDB connection failures

```
[ERROR] [mongo] Failed to connect to MongoDB: connection timeout
```

**Solutions**:

1. Verify MongoDB is running: `systemctl status mongod`
2. Check connection string format in `config.yml`
3. Test connectivity: `mongosh "mongodb://localhost:27017/grain"`
4. Check firewall settings and network access

### Rate Limiting Too Strict

**Problem**: Legitimate clients being rate limited

```
[WARN] [relay-client] WebSocket rate limit exceeded client_id=c1234
```

**Solutions**:

1. Increase `ws_limit` and `ws_burst` in `config.yml`
2. Adjust `event_limit` for publishing clients
3. Monitor client behavior patterns
4. Consider different limits for authenticated users

### Configuration Reload Failures

**Problem**: Configuration changes not taking effect

```
[ERROR] [config] Invalid configuration detected file=config.yml
```

**Solutions**:

1. Validate YAML syntax with online validator
2. Check for proper indentation and structure
3. Verify numeric values are in valid ranges
4. Review error logs for specific validation failures

[def]: #grain-configuration-guide
