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
      - [Derive Public Key from Private Key](#derive-public-key-from-private-key)
      - [Convert Public Key](#convert-public-key)
      - [Convert Private Key](#convert-private-key)
      - [Validate Key](#validate-key)
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
      - [Get Whitelisted Keys (Cached)](#get-whitelisted-keys-cached)
      - [Get Whitelisted Keys (Live)](#get-whitelisted-keys-live)
      - [Get Blacklisted Keys (Cached)](#get-blacklisted-keys-cached)
      - [Get Blacklisted Keys (Live)](#get-blacklisted-keys-live)
      - [Usage Guidelines](#usage-guidelines)
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
GET /api/v1/keys/generate
```

Generates a new random Nostr keypair.

**Response:**

```json
{
  "private_key": "ee5e36081fe74482ce9085a8e97ee020d6d20a0d1fddc0dd986c5629883b111a",
  "public_key": "68027a7931229f043fed19028462df6279fcaf099d33cb75edaf2c5d698b23ad",
  "nsec": "nsec1ae0rvzqluazg9n5ssk5wjlhqyrtdyzsdrlwuphvcd3tznzpmzydqzxfym7",
  "npub": "npub1dqp857f3y20sg0ldrypggcklvfuletcfn5euka0d4uk966vtywksrs7n24"
}
```

#### Derive Public Key from Private Key

```http
GET /api/v1/keys/derive/<private_key>
```

Derives the public key from a private key. Accepts both hex and nsec formats.

**Examples:**

Derive from hex private key:

```http
GET /api/v1/keys/derive/ee5e36081fe74482ce9085a8e97ee020d6d20a0d1fddc0dd986c5629883b111a
```

Derive from nsec:

```http
GET /api/v1/keys/derive/nsec1ae0rvzqluazg9n5ssk5wjlhqyrtdyzsdrlwuphvcd3tznzpmzydqzxfym7
```

**Parameters:**

- `pubkey` (required) - Hex-encoded public key

**Response:**

```json
{
  "public_key": "68027a7931229f043fed19028462df6279fcaf099d33cb75edaf2c5d698b23ad",
  "npub": "npub1dqp857f3y20sg0ldrypggcklvfuletcfn5euka0d4uk966vtywksrs7n24"
}
```

#### Convert Public Key

```http
GET /api/v1/keys/convert/public/<key>
```

Converts between hex and npub formats. Auto-detects input format.

**Examples:**

Convert hex to npub:

```http
GET /api/v1/keys/convert/public/3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d
```

Convert npub to hex:

```http
GET /api/v1/keys/convert/public/npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6
```

**Response:**

Convert hex to npub:

```json
{
  "npub": "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
}
```

Convert npub to hex:

```json
{
  "public_key": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
}
```

#### Convert Private Key

```http
GET /api/v1/keys/convert/private/<key>
```

Converts between hex and nsec formats. Auto-detects input format.

**Examples:**

Convert hex to nsec:

```http
GET /api/v1/keys/convert/private/ee5e36081fe74482ce9085a8e97ee020d6d20a0d1fddc0dd986c5629883b111a
```

Convert nsec to hex:

```http
GET /api/v1/keys/convert/private/nsec1ae0rvzqluazg9n5ssk5wjlhqyrtdyzsdrlwuphvcd3tznzpmzydqzxfym7
```

**Parameters:**

- `pubkey` (required) - Hex-encoded public key to validate

**Response:**

Convert hex to nsec:

```json
{
  "nsec": "nsec1ae0rvzqluazg9n5ssk5wjlhqyrtdyzsdrlwuphvcd3tznzpmzydqzxfym7"
}
```

Convert nsec to hex:

```json
{
  "private_key": "ee5e36081fe74482ce9085a8e97ee020d6d20a0d1fddc0dd986c5629883b111a"
}
```

#### Validate Key

```http
GET /api/v1/keys/validate/<key>
```

Validates any key type (hex, npub, or nsec) and returns the key type.

**Examples:**

Validate hex public key:

```http
GET /api/v1/keys/validate/3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d
```

Validate npub:

```http
GET /api/v1/keys/validate/npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6
```

Validate nsec:

```http
GET /api/v1/keys/validate/nsec1ae0rvzqluazg9n5ssk5wjlhqyrtdyzsdrlwuphvcd3tznzpmzydqzxfym7
```

**Response:**

Valid key:

```json
{
  "valid": true,
  "type": "npub"
}
```

Invalid key:

```json
{
  "valid": false,
  "type": "unknown",
  "error": "Invalid key format"
}
```

**Error Response (for all endpoints):**

```json
{
  "error": "Error message"
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

> **⚠️ Note**: The publish endpoint is currently in development. The current implementation has limited functionality. Full support for all signing methods and anonymous publishing will be available in the next update.

```http
POST /api/v1/publish
```

Publishes a Nostr event to relays with flexible signing options. Supports both authenticated and anonymous publishing.

**Authentication:** Optional - works with or without session

**Signing Methods Available (Next Update):**

**Browser Extension Signing (Recommended)**

```json
{
  "content": "Hello Nostr!",
  "kind": 1,
  "tags": [],
  "signingMethod": "extension",
  "relays": ["wss://relay.damus.io", "wss://nos.lol"]
}
```

**Amber Signer (Android)**

```json
{
  "content": "Hello Nostr!",
  "kind": 1,
  "tags": [],
  "signingMethod": "amber",
  "relays": ["wss://relay.damus.io"]
}
```

**Bunker/Remote Signer**

```json
{
  "content": "Hello Nostr!",
  "kind": 1,
  "tags": [],
  "signingMethod": "bunker",
  "bunkerUri": "bunker://npub1234...@relay.example.com?relay=wss://relay.example.com",
  "relays": ["wss://relay.damus.io"]
}
```

**Anonymous Random Key**

```json
{
  "content": "Hello Nostr!",
  "kind": 1,
  "tags": [],
  "signingMethod": "anonymous"
}
```

**Manual Private Key (Not Recommended)**

```json
{
  "content": "Hello Nostr!",
  "kind": 1,
  "tags": [],
  "signingMethod": "manual",
  "privateKey": "your_private_key_hex",
  "relays": ["wss://relay.damus.io"]
}
```

**Request Fields:**

- `content` (required): Event content text
- `kind` (required): Nostr event kind
- `signingMethod` (required): One of: `extension`, `amber`, `bunker`, `anonymous`, `manual`
- `tags` (optional): Tag arrays `[["e", "event_id"], ["p", "pubkey"]]`
- `relays` (optional): Custom relay URLs
- `privateKey` (conditional): Required for `signingMethod: "manual"`
- `bunkerUri` (conditional): Required for `signingMethod: "bunker"`

**Authentication Behavior:**

- **With Session**: Uses session pubkey and relays
- **Without Session**: Forces anonymous mode with default relays

**Response:**

```json
{
  "success": true,
  "eventId": "1234567890abcdef...",
  "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
  "signingMethod": "extension",
  "event": {
    "id": "1234567890abcdef...",
    "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
    "created_at": 1704067200,
    "kind": 1,
    "content": "Hello Nostr!",
    "tags": [],
    "sig": "abcdef1234567890..."
  },
  "results": [
    {
      "relay": "wss://relay.damus.io",
      "success": true,
      "error": null,
      "responseTime": 150
    }
  ],
  "summary": {
    "successful": 1,
    "failed": 0,
    "totalRelays": 1,
    "successRate": 1.0
  }
}
```

**Error Response:**

```json
{
  "success": false,
  "error": "User denied signing request",
  "signingMethod": "extension",
  "code": "SIGNING_DENIED"
}
```

**Security Notes:**

- Browser extension and remote signers are recommended over manual private keys
- Anonymous mode generates ephemeral keypairs for one-time use
- Manual private key transmission should only occur over HTTPS
- Default relays are used when no session exists

#### Query Events

```http
GET /api/v1/events/query?authors=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d&kinds=1&limit=10
```

Queries events from connected relays using Nostr filters. Uses a 10-second timeout for collecting results.

**Query Parameters:**

- `authors`: Pubkey values (repeat parameter for multiple: `authors=pubkey1&authors=pubkey2`)
- `kinds`: Event kind numbers (repeat parameter for multiple: `kinds=1&kinds=7`)
- `limit`: Maximum number of events to return
- `ids`: Event IDs (repeat parameter for multiple: `ids=id1&ids=id2`)

**Currently Supported Parameters:**

- ✅ `authors` - Filter by pubkey
- ✅ `kinds` - Filter by event kind
- ✅ `limit` - Maximum results
- ✅ `ids` - Filter by event ID

**Not Yet Implemented:**

- ❌ `since` - Unix timestamp (planned)
- ❌ `until` - Unix timestamp (planned)
- ❌ `#e` - Event ID tags (planned)
- ❌ `#p` - Pubkey tags (planned)

**Example Queries:**

Single author, single kind:

```
GET /api/v1/events/query?authors=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d&kinds=1&limit=10
```

Multiple authors and kinds:

```
GET /api/v1/events/query?authors=pubkey1&authors=pubkey2&kinds=1&kinds=7&limit=20
```

Specific event IDs:

```
GET /api/v1/events/query?ids=event_id_1&ids=event_id_2
```

**Response:**

```json
{
  "events": [
    {
      "id": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
      "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
      "created_at": 1234567890,
      "kind": 1,
      "content": "Hello Nostr!",
      "tags": [],
      "sig": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
    }
  ],
  "count": 1
}
```

**Response Fields:**

- `events`: Array of Nostr events matching the filter
- `count`: Number of events returned

**Behavior:**

- Creates WebSocket subscription to fetch events
- Collects results for up to 10 seconds
- Returns when EOSE (End of Stored Events) received or timeout reached
- Uses connected relays from core client

### User Data

#### Get User Profile

```http
GET /api/v1/user/profile?pubkey=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d
```

Fetches user profile metadata (kind 0 event). If no pubkey is provided, uses current session pubkey.

**Response:**

```json
{
  "id": "c43be8b4634298e97dde3020a5e6aeec37d7f5a4b0259705f496e81a550c8f8b",
  "pubkey": "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
  "created_at": 1738588530,
  "kind": 0,
  "tags": [],
  "content": "{\"name\":\"fiatjaf\",\"about\":\"~\",\"picture\":\"https://fiatjaf.com/static/favicon.jpg\",\"nip05\":\"_@fiatjaf.com\",\"lud16\":\"fiatjaf@zbd.gg\",\"website\":\"https://nostr.technology\"}",
  "sig": "202a1bf6a58943d660c1891662dbdda142aa8e5bca9d4a3cb03cde816ad3bdda6f4ec3b880671506c2820285b32218a0afdec2d172de9694d83972190ab4f9da"
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
  "read": ["wss://inbox.relays.land/"],
  "write": ["wss://pyramid.fiatjaf.com/", "wss://relay.westernbtc.com/"],
  "both": null
}
```

---

## Relay Management API

Administrative endpoints for relay operators.

### Key Management

GRAIN provides both **cached** and **live** endpoints for key management. Use cached endpoints for performance and live endpoints for verification after configuration changes.

#### Get Whitelisted Keys (Cached)

```http
GET /api/v1/relay/keys/whitelist
```

Returns all whitelisted pubkeys from cache. **Uses cached data** - may not reflect recent configuration changes until next cache refresh. This endpoint provides fast responses and is suitable for regular operations.

**Cache Refresh Interval:** Based on `cache_refresh_minutes` in whitelist configuration (default: 60 minutes)

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
    }
  ]
}
```

**Response Fields:**

- `list`: All cached pubkeys from all sources (config + domains)
- `domains`: Array of domain objects with their cached pubkeys

#### Get Whitelisted Keys (Live)

```http
GET /api/v1/relay/keys/whitelist/live
```

Returns all whitelisted pubkeys with fresh domain fetching. **Fetches live data** - makes HTTP requests to domains for current NIP-05 data. Use this endpoint to verify configuration changes or when you need the most current data.

**Performance Note:** Slower response due to live HTTP requests to external domains.

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
    }
  ]
}
```

**Response Fields:**

- `list`: All pubkeys from config + live domain fetch
- `domains`: Array of domain objects with freshly fetched pubkeys

#### Get Blacklisted Keys (Cached)

```http
GET /api/v1/relay/keys/blacklist
```

Returns all blacklisted pubkeys organized by source. **Uses live mutelist data** - fetches current mutelist data for consistent structure with live endpoint.

**Response:**

```json
{
  "permanent": [
    "33c74427f3b2b73d5e38f3e6c991c122a55d204072356f71da49a0e209fb6940",
    "db0c9b8acd6101adb9b281c5321f98f6eebb33c5719d230ed1870997538a9765"
  ],
  "temporary": [
    {
      "pubkey": "efgh5678901234567890abcdef1234567890abcdef1234567890abcdef1234",
      "expires_at": 1704067200
    }
  ],
  "mutelist": {
    "16f1a0100d4cfffbcc4230e8e0e4290cc5849c1adc64d6653fda07c031b1074b": [
      "a30816b063c858965f032ee5aa50b6e8091225e583970c2b167ac00de6c54ba5",
      "cf94884ef3330842f55faeeeec6bb3b0e3f0c63ccf2b9ac5f15c94752f637cda"
    ],
    "3fe0ab6cbdb7ee27148202249e3fb3b89423c6f6cda6ef43ea5057c3d93088e4": [
      "0f45cbe562351c7211742fe02cc3e6f91d6cf5b306873c0f3e9fc0c570d3371c",
      "d8a6ecf0c396eaa8f79a4497fe9b77dc977633451f3ca5c634e208659116647b"
    ]
  }
}
```

**Response Fields:**

- `permanent`: Permanently blacklisted pubkeys from config
- `temporary`: Temporarily banned pubkeys with expiration timestamps (null if none)
- `mutelist`: Object where keys are mutelist author pubkeys and values are arrays of their muted pubkeys

#### Get Blacklisted Keys (Live)

```http
GET /api/v1/relay/keys/blacklist/live
```

Returns all blacklisted pubkeys with fresh mutelist fetching. **Fetches live data** - makes WebSocket requests to local relay for current mutelist data. Use this endpoint to verify recent mutelist changes or when you need the most current data.

**Performance Note:** Slower response due to live WebSocket requests to fetch mute lists.

**Response:**

```json
{
  "permanent": [
    "33c74427f3b2b73d5e38f3e6c991c122a55d204072356f71da49a0e209fb6940",
    "db0c9b8acd6101adb9b281c5321f98f6eebb33c5719d230ed1870997538a9765"
  ],
  "temporary": null,
  "mutelist": {
    "16f1a0100d4cfffbcc4230e8e0e4290cc5849c1adc64d6653fda07c031b1074b": [
      "a30816b063c858965f032ee5aa50b6e8091225e583970c2b167ac00de6c54ba5",
      "cf94884ef3330842f55faeeeec6bb3b0e3f0c63ccf2b9ac5f15c94752f637cda"
    ],
    "3fe0ab6cbdb7ee27148202249e3fb3b89423c6f6cda6ef43ea5057c3d93088e4": [
      "0f45cbe562351c7211742fe02cc3e6f91d6cf5b306873c0f3e9fc0c570d3371c",
      "d8a6ecf0c396eaa8f79a4497fe9b77dc977633451f3ca5c634e208659116647b"
    ]
  }
}
```

**Response Fields:**

- `permanent`: Permanently blacklisted pubkeys from config
- `temporary`: Temporarily banned pubkeys with expiration timestamps (null if none)
- `mutelist`: Object where keys are mutelist author pubkeys and values are arrays of their live-fetched muted pubkeys

#### Usage Guidelines

**Use Cached Endpoints When:**

- Regular operations and monitoring
- Building dashboards or UIs
- Need structured breakdown by source
- Want to see mutelist author organization

**Use Live Endpoints When:**

- Verifying configuration changes
- Testing new mutelist author additions
- Debugging blacklist issues
- Need guaranteed current mutelist data

**Configuration Notes:**

- Both endpoints return the same structure for consistency
- Cached endpoint still fetches live mutelist data for proper grouping
- `mutelist` field shows which pubkeys came from which mutelist authors
- Data is returned regardless of blacklist enabled/disabled state
- `temporary` field returns `null` when no temporary bans exist

**Mutelist Structure:**

The `mutelist` object uses mutelist author pubkeys as keys:

- Key: Mutelist author's pubkey (from `mutelist_authors` config)
- Value: Array of pubkeys that author has muted (from their kind 10000 events)

This allows you to see which specific mutelist authors contributed which muted pubkeys to your relay's blacklist.

## Configuration Endpoints

All configuration endpoints are read-only and return current relay settings.

#### Get Server Configuration

```http
GET /api/v1/relay/config/server
```

**Response:**

```json
{
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

| Method | Endpoint                             | Description                        |
| ------ | ------------------------------------ | ---------------------------------- |
| GET    | `/api/v1/session`                    | Get current session info           |
| GET    | `/api/v1/cache`                      | Get cached user data               |
| POST   | `/api/v1/cache/refresh`              | Refresh cache manually             |
| POST   | `/api/v1/auth/login`                 | Login with public key              |
| POST   | `/api/v1/auth/logout`                | Logout current session             |
| GET    | `/api/v1/auth/amber-callback`        | Amber signer callback              |
| GET    | `/api/v1/keys/generate`              | Generate new keypair               |
| GET    | `/api/v1/keys/derive/{private_key}`  | Derive public key from private key |
| GET    | `/api/v1/keys/convert/public/{key}`  | Convert between hex and npub       |
| GET    | `/api/v1/keys/convert/private/{key}` | Convert between hex and nsec       |
| GET    | `/api/v1/keys/validate/{key}`        | Validate any key type              |
| GET    | `/api/v1/relay/ping`                 | Ping relay                         |
| POST   | `/api/v1/relays/connect`             | Connect to relay                   |
| POST   | `/api/v1/relays/disconnect`          | Disconnect from relay              |
| GET    | `/api/v1/relays/status`              | Get relay connection status        |
| POST   | `/api/v1/publish`                    | Publish Nostr event                |
| GET    | `/api/v1/events/query`               | Query events with filters          |
| GET    | `/api/v1/user/profile`               | Get user profile                   |
| GET    | `/api/v1/user/relays`                | Get user relay list                |

### Relay Management API Endpoints

| Method | Endpoint                                      | Description                   |
| ------ | --------------------------------------------- | ----------------------------- |
| GET    | `/api/v1/relay/keys/whitelist`                | Get whitelisted keys (cached) |
| GET    | `/api/v1/relay/keys/whitelist/live`           | Get whitelisted keys (live)   |
| GET    | `/api/v1/relay/keys/blacklist`                | Get blacklisted keys (cached) |
| GET    | `/api/v1/relay/keys/blacklist/live`           | Get blacklisted keys (live)   |
| GET    | `/api/v1/relay/config/server`                 | Get server configuration      |
| GET    | `/api/v1/relay/config/rate_limit`             | Get rate limit configuration  |
| GET    | `/api/v1/relay/config/event_purge`            | Get event purge configuration |
| GET    | `/api/v1/relay/config/logging`                | Get logging configuration     |
| GET    | `/api/v1/relay/config/mongodb`                | Get MongoDB configuration     |
| GET    | `/api/v1/relay/config/resource_limits`        | Get resource limits           |
| GET    | `/api/v1/relay/config/auth`                   | Get auth configuration        |
| GET    | `/api/v1/relay/config/event_time_constraints` | Get time constraints          |
| GET    | `/api/v1/relay/config/backup_relay`           | Get backup relay config       |
| GET    | `/api/v1/relay/config/user_sync`              | Get user sync config          |
| GET    | `/api/v1/relay/config/whitelist`              | Get complete whitelist config |
| GET    | `/api/v1/relay/config/blacklist`              | Get complete blacklist config |

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
