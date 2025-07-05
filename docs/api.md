# GRAIN API Documentation

Complete REST API reference for GRAIN relay operations.

## Table of Contents

- [GRAIN API Documentation](#grain-api-documentation)
  - [Table of Contents](#table-of-contents)
  - [Base URL](#base-url)
  - [Client API Endpoints](#client-api-endpoints)
    - [Session Management](#session-management)
      - [Get Current Session](#get-current-session)
    - [Cache Management](#cache-management)
      - [Get Cache Data](#get-cache-data)
      - [Refresh Cache](#refresh-cache)
    - [Authentication](#authentication)
      - [Login](#login)
      - [Logout](#logout)
      - [Amber Callback (NIP-55)](#amber-callback-nip-55)
    - [Key Operations](#key-operations)
      - [Generate Keypair](#generate-keypair)
      - [Convert Public Key to npub](#convert-public-key-to-npub)
      - [Convert npub to Public Key](#convert-npub-to-public-key)
      - [Validate Public Key](#validate-public-key)
      - [Validate npub](#validate-npub)
    - [Relay Operations](#relay-operations)
      - [Ping Relay](#ping-relay)
      - [Connect to Relay](#connect-to-relay)
      - [Disconnect from Relay](#disconnect-from-relay)
      - [Get Relay Status](#get-relay-status)
    - [Event Operations](#event-operations)
      - [Publish Event](#publish-event)
      - [Query Events](#query-events)
    - [User Data](#user-data)
      - [Get User Profile](#get-user-profile)
      - [Get User Relays](#get-user-relays)
  - [Relay Management API](#relay-management-api)
    - [Key Management](#key-management)
      - [Get Whitelisted Keys](#get-whitelisted-keys)
      - [Get Blacklisted Keys](#get-blacklisted-keys)
    - [Configuration Endpoints](#configuration-endpoints)
      - [Get Server Configuration](#get-server-configuration)
      - [Get Rate Limit Configuration](#get-rate-limit-configuration)
      - [Get Event Purge Configuration](#get-event-purge-configuration)
      - [Get Logging Configuration](#get-logging-configuration)
      - [Get MongoDB Configuration](#get-mongodb-configuration)
      - [Get Resource Limits Configuration](#get-resource-limits-configuration)
      - [Get Auth Configuration](#get-auth-configuration)
      - [Get Event Time Constraints Configuration](#get-event-time-constraints-configuration)
      - [Get Backup Relay Configuration](#get-backup-relay-configuration)
      - [Get User Sync Configuration](#get-user-sync-configuration)
      - [Get Whitelist Configuration](#get-whitelist-configuration)
      - [Get Blacklist Configuration](#get-blacklist-configuration)
  - [WebSocket \& Protocol Endpoints](#websocket--protocol-endpoints)
    - [Nostr WebSocket Relay](#nostr-websocket-relay)
    - [NIP-11 Relay Information](#nip-11-relay-information)
  - [Progressive Web App (PWA) Endpoints](#progressive-web-app-pwa-endpoints)
    - [Web App Manifest](#web-app-manifest)
    - [Service Worker](#service-worker)
  - [Quick Reference](#quick-reference)
    - [Client API Endpoints](#client-api-endpoints-1)
    - [Relay Management API Endpoints](#relay-management-api-endpoints)
    - [Other Endpoints](#other-endpoints)
  - [Response Status Codes](#response-status-codes)
  - [Error Response Format](#error-response-format)
  - [Rate Limiting](#rate-limiting)

## Base URL

```
/api/v1
```

## Client API Endpoints

Web client operations, user management, and Nostr client functionality.

### Session Management

#### Get Current Session

```http
GET /api/v1/session
```

Returns information about the current user session.

**Response:**

```json
{
  "publicKey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
  "lastActive": "2024-01-15T12:00:00Z",
  "relays": {
    "userRelays": ["wss://relay.damus.io", "wss://nos.lol"],
    "relayCount": 2
  }
}
```

### Cache Management

#### Get Cache Data

```http
GET /api/v1/cache
```

Returns cached user data including profile and mailboxes.

**Response:**

```json
{
  "publicKey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
  "npub": "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
  "metadata": {
    "name": "fiatjaf",
    "about": "I made Nostr",
    "picture": "https://example.com/pic.jpg"
  },
  "mailboxes": {
    "read": ["wss://relay.damus.io"],
    "write": ["wss://nos.lol"],
    "both": ["wss://relay.nostr.band"]
  }
}
```

#### Refresh Cache

```http
POST /api/v1/cache/refresh
```

Manually triggers a cache refresh for the current user.

**Response:**

```json
{
  "status": "success",
  "message": "Cache refreshed successfully"
}
```

### Authentication

#### Login

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "publicKey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
}
```

**Response:**

```json
{
  "status": "success",
  "publicKey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
}
```

#### Logout

```http
POST /api/v1/auth/logout
```

**Response:**

```json
{
  "status": "success",
  "message": "Logged out successfully"
}
```

#### Amber Callback (NIP-55)

```http
GET /api/v1/auth/amber-callback?event={signed_event}
```

Handles the callback from Amber signer for NIP-55 authentication flow.

### Key Operations

#### Generate Keypair

```http
GET /api/v1/generate/keypair
```

Generates a new random Nostr keypair.

**Response:**

```json
{
  "privateKey": "nsec1...",
  "publicKey": "npub1..."
}
```

#### Convert Public Key to npub

```http
POST /api/v1/convert/pubkey
Content-Type: application/json

{
  "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
}
```

**Response:**

```json
{
  "npub": "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
}
```

#### Convert npub to Public Key

```http
POST /api/v1/convert/npub
Content-Type: application/json

{
  "npub": "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
}
```

**Response:**

```json
{
  "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
}
```

#### Validate Public Key

```http
POST /api/v1/validate/pubkey
Content-Type: application/json

{
  "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
}
```

**Response:**

```json
{
  "valid": true
}
```

#### Validate npub

```http
POST /api/v1/validate/npub
Content-Type: application/json

{
  "npub": "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
}
```

**Response:**

```json
{
  "valid": true
}
```

### Relay Operations

#### Ping Relay

```http
GET /api/v1/relay/ping
```

Checks if the local relay is responsive.

**Response:**

```json
{
  "status": "pong",
  "timestamp": "2024-01-15T12:00:00Z"
}
```

#### Connect to Relay

```http
POST /api/v1/relays/connect
Content-Type: application/json

{
  "url": "wss://relay.damus.io"
}
```

**Response:**

```json
{
  "status": "connected",
  "relay": "wss://relay.damus.io"
}
```

#### Disconnect from Relay

```http
POST /api/v1/relays/disconnect
Content-Type: application/json

{
  "url": "wss://relay.damus.io"
}
```

**Response:**

```json
{
  "status": "disconnected",
  "relay": "wss://relay.damus.io"
}
```

#### Get Relay Status

```http
GET /api/v1/relays/status
```

**Response:**

```json
{
  "relays": [
    {
      "url": "wss://relay.damus.io",
      "status": "connected",
      "latency": 45
    },
    {
      "url": "wss://nos.lol",
      "status": "connected",
      "latency": 120
    }
  ]
}
```

### Event Operations

#### Publish Event

```http
POST /api/v1/publish
Content-Type: application/json

{
  "content": "Hello Nostr!",
  "kind": 1,
  "tags": []
}
```

**Response:**

```json
{
  "event_id": "1234567890abcdef...",
  "relay_status": {
    "wss://relay.damus.io": "success",
    "wss://nos.lol": "success"
  }
}
```

#### Query Events

```http
GET /api/v1/events/query?authors=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d&kinds=1&limit=10
```

**Query Parameters:**

- `authors`: Comma-separated list of pubkeys
- `kinds`: Comma-separated list of event kinds
- `since`: Unix timestamp
- `until`: Unix timestamp
- `limit`: Maximum number of events
- `#e`: Event IDs referenced in 'e' tags
- `#p`: Pubkeys referenced in 'p' tags

**Response:**

```json
{
  "events": [
    {
      "id": "...",
      "pubkey": "...",
      "created_at": 1234567890,
      "kind": 1,
      "content": "Hello Nostr!",
      "tags": [],
      "sig": "..."
    }
  ]
}
```

### User Data

#### Get User Profile

```http
GET /api/v1/user/profile?pubkey=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d
```

Fetches user profile metadata (kind 0 event). If no pubkey is provided, uses current session pubkey.

**Response:**

```json
{
  "name": "fiatjaf",
  "about": "I made Nostr",
  "picture": "https://example.com/pic.jpg",
  "nip05": "fiatjaf@fiatjaf.com"
}
```

#### Get User Relays

```http
GET /api/v1/user/relays?pubkey=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d
```

Fetches user's relay list (kind 10002 event). If no pubkey is provided, uses current session pubkey.

**Response:**

```json
{
  "read": ["wss://relay.damus.io", "wss://relay.nostr.band"],
  "write": ["wss://nos.lol", "wss://relay.damus.io"],
  "both": ["wss://relay.snort.social"]
}
```

---

## Relay Management API

Administrative endpoints for relay operators.

### Key Management

#### Get Whitelisted Keys

```http
GET /api/v1/relay/keys/whitelist
```

Returns all whitelisted pubkeys organized by source. This endpoint always returns configuration data regardless of whether whitelisting is enabled.

**Response:**

```json
{
  "list": [
    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
    "fa984bd7dbb282f07e16e7ae87b26a2a7b9b90b7246a44771f0cf5ae58018f52"
  ],
  "domains": [
    {
      "domain": "example.com",
      "pubkeys": [
        "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
      ]
    },
    {
      "domain": "nostr.example.org",
      "pubkeys": [
        "567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234"
      ]
    }
  ]
}
```

**Response Fields:**

- `list`: All pubkeys from configuration (direct hex pubkeys + npubs converted to hex)
- `domains`: Array of domain objects with fetched NIP-05 verified pubkeys

#### Get Blacklisted Keys

```http
GET /api/v1/relay/keys/blacklist
```

Returns all blacklisted pubkeys organized by type. This endpoint always returns configuration data regardless of whether blacklisting is enabled.

**Response:**

```json
{
  "permanent": [
    "abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"
  ],
  "temporary": [
    {
      "pubkey": "efgh5678901234567890abcdef1234567890abcdef1234567890abcdef1234",
      "expires_at": 1704067200
    }
  ],
  "mutelist": {
    "author_pubkey_1": ["blocked_pubkey_1", "blocked_pubkey_2"],
    "author_pubkey_2": ["blocked_pubkey_3"]
  }
}
```

**Response Fields:**

- `permanent`: Pubkeys permanently blacklisted (from config)
- `temporary`: Pubkeys temporarily banned with expiration timestamps
- `mutelist`: Pubkeys blocked via NIP-51 mute lists, grouped by list author

### Configuration Endpoints

All configuration endpoints are read-only and return current relay settings.

#### Get Server Configuration

```http
GET /api/v1/relay/config/server
```

**Response:**

```json
{
  "port": ":8080",
  "read_timeout": 10,
  "write_timeout": 10,
  "idle_timeout": 120,
  "max_subscriptions_per_client": 10,
  "implicit_req_limit": 100
}
```

#### Get Rate Limit Configuration

```http
GET /api/v1/relay/config/rate_limit
```

**Response:**

```json
{
  "ws_limit": 10,
  "ws_burst": 5,
  "event_limit": 100,
  "event_burst": 10,
  "req_limit": 50,
  "req_burst": 10,
  "max_event_size": 65536,
  "kind_size_limits": [
    {
      "kind": 0,
      "max_size": 8192
    }
  ],
  "category_limits": {
    "regular": {
      "rate": 100,
      "burst": 10
    }
  },
  "kind_limits": [
    {
      "kind": 1,
      "rate": 50,
      "burst": 5
    }
  ]
}
```

#### Get Event Purge Configuration

```http
GET /api/v1/relay/config/event_purge
```

**Response:**

```json
{
  "enabled": true,
  "disable_at_startup": false,
  "keep_interval_hours": 24,
  "purge_interval_minutes": 60,
  "purge_by_category": {
    "regular": true,
    "replaceable": false,
    "ephemeral": true
  },
  "purge_by_kind_enabled": true,
  "kinds_to_purge": [1, 7],
  "exclude_whitelisted": true
}
```

#### Get Logging Configuration

```http
GET /api/v1/relay/config/logging
```

**Response:**

```json
{
  "level": "info",
  "file": "debug",
  "max_log_size_mb": 10,
  "structure": false,
  "check_interval_min": 10,
  "backup_count": 2,
  "suppress_components": [
    "util",
    "conn-manager",
    "client",
    "mongo-query",
    "event-store",
    "close-handler"
  ]
}
```

#### Get MongoDB Configuration

```http
GET /api/v1/relay/config/mongodb
```

**Response:**

```json
{
  "uri": "mongodb://localhost:27017",
  "database": "grain"
}
```

#### Get Resource Limits Configuration

```http
GET /api/v1/relay/config/resource_limits
```

**Response:**

```json
{
  "cpu_cores": 4,
  "memory_mb": 1024,
  "heap_size_mb": 512
}
```

#### Get Auth Configuration

```http
GET /api/v1/relay/config/auth
```

**Response:**

```json
{
  "enabled": true,
  "relay_url": "wss://auth.relay.com"
}
```

#### Get Event Time Constraints Configuration

```http
GET /api/v1/relay/config/event_time_constraints
```

**Response:**

```json
{
  "max_created_at_future_seconds": 900,
  "max_created_at_past_seconds": 94608000
}
```

#### Get Backup Relay Configuration

```http
GET /api/v1/relay/config/backup_relay
```

**Response:**

```json
{
  "enabled": true,
  "url": "wss://backup.relay.com"
}
```

#### Get User Sync Configuration

```http
GET /api/v1/relay/config/user_sync
```

**Response:**

```json
{
  "enabled": true,
  "interval_hours": 6,
  "batch_size": 100
}
```

#### Get Whitelist Configuration

```http
GET /api/v1/relay/config/whitelist
```

Returns the complete whitelist configuration from whitelist.yml.

**Response:**

```json
{
  "pubkey_whitelist": {
    "enabled": false,
    "pubkeys": [
      "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
    ],
    "npubs": [
      "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
    ],
    "cache_refresh_minutes": 60
  },
  "kind_whitelist": {
    "enabled": false,
    "kinds": ["0", "1", "3", "7"]
  },
  "domain_whitelist": {
    "enabled": false,
    "domains": ["example.com", "nostr.example.org"],
    "cache_refresh_minutes": 120
  }
}
```

#### Get Blacklist Configuration

```http
GET /api/v1/relay/config/blacklist
```

Returns the complete blacklist configuration from blacklist.yml.

**Response:**

```json
{
  "enabled": true,
  "permanent_ban_words": ["spam", "scam"],
  "temp_ban_words": ["crypto", "airdrop"],
  "max_temp_bans": 3,
  "temp_ban_duration": 3600,
  "permanent_blacklist_pubkeys": [
    "abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"
  ],
  "permanent_blacklist_npubs": [
    "npub1abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
  ],
  "mutelist_authors": ["author_pubkey_1", "author_pubkey_2"],
  "mutelist_cache_refresh_minutes": 120
}
```

---

## WebSocket & Protocol Endpoints

### Nostr WebSocket Relay

```
ws://localhost:8080/
wss://yourdomain.com/
```

Standard Nostr relay protocol implementation supporting NIPs 1, 2, 9, 11, 40, 42.

### NIP-11 Relay Information

```http
GET /
Accept: application/nostr+json
```

**Response:**

```json
{
  "name": "🌾 My GRAIN Relay",
  "description": "A community Nostr relay running GRAIN",
  "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
  "contact": "admin@example.com",
  "supported_nips": [1, 2, 9, 11, 40, 42],
  "software": "https://github.com/0ceanslim/grain",
  "version": "0.4.0",
  "limitation": {
    "max_message_length": 65536,
    "max_subscriptions": 10,
    "max_filters": 10,
    "max_limit": 5000,
    "payment_required": false,
    "auth_required": false
  }
}
```

---

## Progressive Web App (PWA) Endpoints

### Web App Manifest

```http
GET /manifest.json
```

Returns the PWA manifest for installable web app functionality.

### Service Worker

```http
GET /sw.js
```

Returns the service worker JavaScript for offline functionality.

---

## Quick Reference

### Client API Endpoints

| Method | Endpoint                      | Description                 |
| ------ | ----------------------------- | --------------------------- |
| GET    | `/api/v1/session`             | Get current session info    |
| GET    | `/api/v1/cache`               | Get cached user data        |
| POST   | `/api/v1/cache/refresh`       | Refresh cache manually      |
| POST   | `/api/v1/auth/login`          | Login with public key       |
| POST   | `/api/v1/auth/logout`         | Logout current session      |
| GET    | `/api/v1/auth/amber-callback` | Amber signer callback       |
| GET    | `/api/v1/generate/keypair`    | Generate new keypair        |
| POST   | `/api/v1/convert/pubkey`      | Convert pubkey to npub      |
| POST   | `/api/v1/convert/npub`        | Convert npub to pubkey      |
| POST   | `/api/v1/validate/pubkey`     | Validate public key         |
| POST   | `/api/v1/validate/npub`       | Validate npub               |
| GET    | `/api/v1/relay/ping`          | Ping relay                  |
| POST   | `/api/v1/relays/connect`      | Connect to relay            |
| POST   | `/api/v1/relays/disconnect`   | Disconnect from relay       |
| GET    | `/api/v1/relays/status`       | Get relay connection status |
| POST   | `/api/v1/publish`             | Publish Nostr event         |
| GET    | `/api/v1/events/query`        | Query events with filters   |
| GET    | `/api/v1/user/profile`        | Get user profile            |
| GET    | `/api/v1/user/relays`         | Get user relay list         |

### Relay Management API Endpoints

| Method | Endpoint                                      | Description                    |
| ------ | --------------------------------------------- | ------------------------------ |
| GET    | `/api/v1/relay/keys/whitelist`                | Get whitelisted keys by source |
| GET    | `/api/v1/relay/keys/blacklist`                | Get blacklisted keys by type   |
| GET    | `/api/v1/relay/config/server`                 | Get server configuration       |
| GET    | `/api/v1/relay/config/rate_limit`             | Get rate limit configuration   |
| GET    | `/api/v1/relay/config/event_purge`            | Get event purge configuration  |
| GET    | `/api/v1/relay/config/logging`                | Get logging configuration      |
| GET    | `/api/v1/relay/config/mongodb`                | Get MongoDB configuration      |
| GET    | `/api/v1/relay/config/resource_limits`        | Get resource limits            |
| GET    | `/api/v1/relay/config/auth`                   | Get auth configuration         |
| GET    | `/api/v1/relay/config/event_time_constraints` | Get time constraints           |
| GET    | `/api/v1/relay/config/backup_relay`           | Get backup relay config        |
| GET    | `/api/v1/relay/config/user_sync`              | Get user sync config           |
| GET    | `/api/v1/relay/config/whitelist`              | Get complete whitelist config  |
| GET    | `/api/v1/relay/config/blacklist`              | Get complete blacklist config  |

### Other Endpoints

| Method    | Endpoint         | Description              |
| --------- | ---------------- | ------------------------ |
| WebSocket | `/`              | Nostr relay WebSocket    |
| GET       | `/`              | NIP-11 relay information |
| GET       | `/manifest.json` | PWA manifest             |
| GET       | `/sw.js`         | Service worker           |

---

## Response Status Codes

- `200 OK` - Request successful
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication required
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

## Error Response Format

```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": "Additional error details"
}
```

## Rate Limiting

API endpoints are subject to rate limiting based on the relay configuration. Rate limit headers are included in responses:

- `X-RateLimit-Limit`: Maximum requests allowed
- `X-RateLimit-Remaining`: Remaining requests in current window
- `X-RateLimit-Reset`: Unix timestamp when limit resets
