# GRAIN API Documentation WIP

REST API endpoints for GRAIN relay operations.

## Base URL

```
/api/v1
```

## Client API

Web client operations and user management.

### Session Management

#### Get Current Session

```http
GET /api/v1/session
```

**Response:**

```json
{
  "publicKey": "3bf0c...",
  "lastActive": "2024-01-15T12:00:00Z",
  "relays": {
    "userRelays": ["wss://relay.damus.io", "wss://nos.lol"],
    "relayCount": 2
  }
}
```

#### Get Cached User Data

```http
GET /api/v1/cache
```

**Response:**

```json
{
  "publicKey": "3bf0c...",
  "npub": "npub180cvv...",
  "metadata": {
    /* user profile event */
  },
  "mailboxes": {
    "read": ["wss://relay.damus.io"],
    "write": ["wss://nos.lol"]
  }
}
```

---

## Relay Management API

Administrative functions for relay operators.

### Whitelist Management

#### Get Whitelisted Pubkeys

```http
GET /api/v1/whitelist/pubkeys
```

**Response:**

```json
{
  "pubkeys": [
    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
    "fa984bd7dbb282f07e16e7ae87b26a2a7b9b90b7246a44771f0cf5ae58018f52"
  ]
}
```

### Blacklist Management

#### Get Blacklisted Pubkeys

```http
GET /api/v1/blacklist/pubkeys
```

**Response:**

```json
{
  "permanent": ["abcd1234..."],
  "temporary": [
    {
      "pubkey": "efgh5678...",
      "expires_at": 1704067200
    }
  ],
  "mutelist": {
    "author_pubkey": ["blocked_pubkey1", "blocked_pubkey2"]
  }
}
```

---

## Authentication Endpoints

User login/logout flow.

### Login

```http
POST /login
Content-Type: application/x-www-form-urlencoded

publicKey=3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d
```

**Response:** Redirect to `/profile` or error message

### Logout

```http
GET /logout
```

**Response:** Redirect to `/` and clear session

---

## Nostr Protocol

### WebSocket Relay

```
ws://localhost:8181/
```

Standard Nostr relay protocol (NIPs 1, 2, 9, 11).

### NIP-11 Relay Info

```http
GET /
Accept: application/nostr+json
```

**Response:**

```json
{
  "name": "🌾 My GRAIN Relay",
  "description": "A community Nostr relay",
  "supported_nips": [1, 2, 9, 11, 40, 42],
  "software": "https://github.com/0ceanslim/grain",
  "version": "0.4.0"
}
```
