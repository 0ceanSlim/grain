# Grain Client Library Guide

Grain is a powerful Nostr client library built in Go that provides everything you need to build robust Nostr applications. It includes connection management, subscription handling, event building/signing, and authentication flows.

## Table of Contents

- [Overview](#overview)
- [Core Components](#core-components)
- [Quick Start](#quick-start)
- [Authentication & Sessions](#authentication--sessions)
- [Relay Management](#relay-management)
- [Subscriptions & Event Retrieval](#subscriptions--event-retrieval)
- [Event Creation & Publishing](#event-creation--publishing)
- [Key Management](#key-management)
- [Advanced Features](#advanced-features)
- [API Reference](#api-reference)
- [Examples](#examples)

## Overview

Grain provides a complete Nostr client implementation with:

- **Connection pooling**: Efficient management of WebSocket connections to multiple relays
- **Event subscriptions**: Real-time event streaming with filtering
- **Event publishing**: Build, sign, and broadcast events to relays
- **User authentication**: Session management with multiple signing methods
- **Key management**: Generate, derive, and convert Nostr keys
- **Relay discovery**: Automatic relay list fetching and management
- **Caching**: User data and relay information caching

## Core Components

### 1. Client Core (`client/core/`)

The heart of the library, providing the main `Client` struct and core functionality:

- **`client.go`**: Main client with connection management
- **`relays.go`**: WebSocket relay pool and connection handling  
- **`subscriptions.go`**: Event subscription management
- **`config.go`**: Client configuration
- **`eventBuilder.go`**: Fluent API for building events
- **`eventBroadcaster.go`**: Event publishing and broadcasting
- **`eventSigner.go`**: Event signing utilities
- **`eventSerializer.go`**: Event serialization helpers

### 2. Shared Types (`server/types/`)

Core Nostr data structures used throughout the library:

- **`event.go`**: Nostr event structure
- **`filter.go`**: Event filtering for subscriptions
- **`subscription.go`**: Basic subscription structure

### 3. API Layer (`client/api/`)

HTTP API endpoints for web applications:

- **`login.go`**: User authentication
- **`session.go`**: Session management
- **`keysGenerate.go`**: Key pair generation
- **Authentication flow handlers**
- **Event and relay management endpoints**

### 4. Utilities (`client/core/tools/`)

Helper functions for Nostr operations:

- **Key pair generation and conversion**
- **NIP-19 encoding/decoding (npub, nsec, etc.)**
- **Public key derivation**

## Quick Start

### Basic Client Setup

```go
package main

import (
    "log"
    "github.com/0ceanslim/grain/client/core"
)

func main() {
    // Create client with default configuration
    config := core.DefaultConfig()
    client := core.NewClient(config)
    
    // Connect to relays
    relays := []string{
        "wss://relay.damus.io",
        "wss://nos.lol", 
        "wss://relay.nostr.band",
    }
    
    if err := client.ConnectToRelays(relays); err != nil {
        log.Fatal("Failed to connect:", err)
    }
    
    log.Println("Connected to", len(client.GetConnectedRelays()), "relays")
    
    // Don't forget to cleanup
    defer client.Close()
}
```

### Custom Configuration

```go
config := &core.Config{
    DefaultRelays: []string{
        "wss://your-relay.com",
        "wss://another-relay.com",
    },
    ConnectionTimeout: 15 * time.Second,
    ReadTimeout:       45 * time.Second,
    WriteTimeout:      15 * time.Second,
    MaxConnections:    20,
    RetryAttempts:     5,
    RetryDelay:        3 * time.Second,
    UserAgent:         "my-nostr-client/1.0",
}

client := core.NewClient(config)
```

## Authentication & Sessions

Grain provides a comprehensive authentication system supporting multiple signing methods:

### Session Modes

- **Read-Only Mode**: Query events and user data without signing capability
- **Write Mode**: Full event creation and publishing with signing

### Signing Methods

- **No Signing**: Read-only access
- **NIP-07 Browser Extension**: Uses browser extension for signing
- **Private Key**: Direct private key signing
- **Amber (Android)**: External signing via Amber app

### Login Flow Example

```go
import "github.com/0ceanslim/grain/client/session"

// Create session request
loginReq := session.SessionInitRequest{
    PublicKey:      "your-pubkey-hex",
    RequestedMode:  session.WriteMode,
    SigningMethod:  session.PrivateKeySigning,
    PrivateKey:     "your-private-key-hex", // Only for private key signing
}

// This would typically be called via HTTP API
userSession, err := session.CreateUserSession(responseWriter, loginReq)
if err != nil {
    log.Fatal("Login failed:", err)
}

log.Printf("Logged in as %s in %s mode", userSession.PublicKey, userSession.Mode)
```

## Relay Management

### Basic Connection Management

```go
// Connect to multiple relays with retry
err := client.ConnectToRelaysWithRetry(relays, 3)
if err != nil {
    log.Printf("Some connections failed: %v", err)
}

// Get connected relays
connected := client.GetConnectedRelays()
log.Printf("Connected to %d relays: %v", len(connected), connected)

// Disconnect from specific relay
err = client.DisconnectFromRelay("wss://relay.example.com")
```

### User Relay Discovery

```go
// Fetch user's relay list (kind 10002)
mailboxes, err := client.GetUserRelays("pubkey-hex")
if err != nil {
    log.Printf("Failed to get user relays: %v", err)
} else {
    log.Printf("Read relays: %v", mailboxes.Read)
    log.Printf("Write relays: %v", mailboxes.Write) 
    log.Printf("Both relays: %v", mailboxes.Both)
}

// Switch client to use user's relays
relayConfigs := []core.RelayConfig{
    {URL: "wss://user-relay1.com", Read: true, Write: true},
    {URL: "wss://user-relay2.com", Read: true, Write: false},
}
err = client.SwitchToUserRelays(relayConfigs)
```

## Subscriptions & Event Retrieval

### Creating Subscriptions

```go
import nostr "github.com/0ceanslim/grain/server/types"

// Create filters for subscription
filters := []nostr.Filter{
    {
        Authors: []string{"pubkey1", "pubkey2"},
        Kinds:   []int{1}, // Text notes
        Limit:   &[]int{50}[0],
    },
}

// Subscribe with specific relays (or nil for all connected)
sub, err := client.Subscribe(filters, nil)
if err != nil {
    log.Fatal("Subscription failed:", err)
}
defer sub.Close()

// Process events
for {
    select {
    case event := <-sub.Events:
        log.Printf("Received event: %s from %s", event.ID, event.PubKey)
        
    case relayURL := <-sub.EOSE:
        log.Printf("End of stored events from %s", relayURL)
        
    case err := <-sub.Errors:
        log.Printf("Subscription error: %v", err)
        
    case <-sub.Done:
        log.Println("Subscription closed")
        return
    }
}
```

### Fetching User Profile

```go
// Get user metadata (kind 0)
profile, err := client.GetUserProfile("pubkey-hex", nil)
if err != nil {
    log.Printf("Failed to get profile: %v", err)
} else {
    log.Printf("Profile: %s", profile.Content) // JSON metadata
}
```

### Advanced Filtering

```go
// Complex filter with time range and tags
since := time.Now().Add(-24 * time.Hour)
until := time.Now()

filters := []nostr.Filter{
    {
        Kinds: []int{1, 6, 7}, // Notes, reposts, reactions
        Tags: map[string][]string{
            "t": {"bitcoin", "nostr"}, // Hashtags
        },
        Since: &since,
        Until: &until,
        Limit: &[]int{100}[0],
    },
}
```

## Event Creation & Publishing

### Building Events

```go
// Use the fluent builder API
textNote := core.NewTextNote("Hello Nostr!")
    .PTag("pubkey-to-mention", "wss://relay-hint.com")
    .TTag("hello")
    .TTag("nostr")

// Build the event (not signed yet)
event := textNote.Build()
```

### Event Signing

```go
// Create signer with private key
signer, err := core.NewPrivateKeySigner("your-private-key-hex")
if err != nil {
    log.Fatal("Failed to create signer:", err)
}

// Sign the event
err = signer.SignEvent(event)
if err != nil {
    log.Fatal("Failed to sign event:", err)
}

log.Printf("Signed event ID: %s", event.ID)
```

### Publishing Events

```go
// Publish to specific relays
targetRelays := []string{"wss://relay1.com", "wss://relay2.com"}
results, err := client.PublishEvent(event, targetRelays)
if err != nil {
    log.Fatal("Publish failed:", err)
}

// Check results
for _, result := range results {
    if result.Success {
        log.Printf("✓ Published to %s", result.RelayURL)
    } else {
        log.Printf("✗ Failed %s: %v", result.RelayURL, result.Error)
    }
}
```

### High-Level Publishing

```go
// Build, sign, and publish in one call
event, results, err := core.PublishEvent(client, signer, textNote, nil)
if err != nil {
    log.Fatal("Publish failed:", err)
}

// Get summary
summary := core.SummarizeBroadcast(results)
log.Printf("Published to %d/%d relays (%.1f%% success)", 
    summary.Successful, summary.TotalRelays, summary.SuccessRate)
```

## Key Management

### Generating Key Pairs

```go
import "github.com/0ceanslim/grain/client/core/tools"

// Generate new key pair
keyPair, err := tools.GenerateKeyPair()
if err != nil {
    log.Fatal("Key generation failed:", err)
}

log.Printf("Private key: %s", keyPair.PrivateKey) // hex
log.Printf("Public key: %s", keyPair.PublicKey)   // hex
log.Printf("nsec: %s", keyPair.Nsec)             // bech32
log.Printf("npub: %s", keyPair.Npub)             // bech32
```

### Key Conversion

```go
// Derive public key from private key
pubkey, err := tools.DerivePubkey("private-key-hex")

// Convert to bech32 formats
nsec, err := tools.EncodePrivkey("private-key-hex")
npub, err := tools.EncodePubkey("public-key-hex")

// Decode bech32 formats
privkey, err := tools.DecodeNsec("nsec1...")
pubkey, err := tools.DecodeNpub("npub1...")
```

## Advanced Features

### Connection Retry Logic

```go
// Connect with automatic retries
err := client.ConnectToRelaysWithRetry(relays, 5) // 5 retry attempts
```

### Publish with Retry

```go
// Publish with retry for failed relays
results, err := client.PublishEventWithRetry(event, relays, 3)
```

### Dynamic Relay Switching

```go
// Switch to user's discovered relays
err := client.ReplaceRelayConnections(userRelayConfigs)

// Switch back to default app relays
err := client.SwitchToDefaultRelays()
```

### Event Builders for Different Kinds

```go
// Reaction (kind 7)
reaction := core.NewReaction("event-id-to-react-to", "🚀")

// Repost (kind 6) 
repost := core.NewRepost("event-id-to-repost", "wss://relay-hint.com")

// Deletion (kind 5)
deletion := core.NewDeletion([]string{"event-id1", "event-id2"}, "spam")

// Contact list (kind 3)
contacts := core.NewContactList()
    .PTag("friend1-pubkey", "wss://their-relay.com")
    .PTag("friend2-pubkey")

// Relay list (kind 10002)
relayList := core.NewRelayList()
    .RTag("wss://my-read-relay.com", "read")
    .RTag("wss://my-write-relay.com", "write")
    .RTag("wss://my-general-relay.com", "")

// Profile/Metadata (kind 0)
profile := core.NewProfile()
    .Content(`{"name":"Alice","about":"Nostr enthusiast","picture":"https://..."}`)
```

## API Reference

### Core Client Methods

#### Connection Management
- `NewClient(config *Config) *Client` - Create new client instance
- `ConnectToRelays(urls []string) error` - Connect to relay list
- `ConnectToRelaysWithRetry(urls []string, maxRetries int) error` - Connect with retry
- `DisconnectFromRelay(url string) error` - Disconnect from specific relay
- `GetConnectedRelays() []string` - Get list of connected relay URLs
- `GetRelayStatus() map[string]string` - Get detailed relay status
- `Close() error` - Close all connections and cleanup

#### Subscription Management  
- `Subscribe(filters []Filter, relayHints []string) (*Subscription, error)` - Create subscription
- `GetUserProfile(pubkey string, relayHints []string) (*Event, error)` - Fetch user profile
- `GetUserRelays(pubkey string) (*Mailboxes, error)` - Get user's relay list

#### Event Publishing
- `PublishEvent(event *Event, targetRelays []string) ([]BroadcastResult, error)` - Publish event
- `PublishEventWithRetry(event *Event, targetRelays []string, maxRetries int) ([]BroadcastResult, error)` - Publish with retry

#### Relay Management
- `ReplaceRelayConnections(newRelays []RelayConfig) error` - Replace all relay connections  
- `SwitchToUserRelays(userRelays []RelayConfig) error` - Switch to user's relays
- `SwitchToDefaultRelays() error` - Switch back to default relays

### Subscription Methods

- `Start() error` - Begin receiving events
- `Close() error` - Stop subscription and cleanup
- `AddRelay(url string) error` - Add relay to active subscription
- `RemoveRelay(url string) error` - Remove relay from subscription
- `IsActive() bool` - Check if subscription is active
- `GetRelayCount() int` - Get number of relays in subscription

### Event Builder Methods

#### Core Builder Methods
- `Content(content string) *EventBuilder` - Set event content
- `Tag(name string, values ...string) *EventBuilder` - Add generic tag
- `CreatedAt(t time.Time) *EventBuilder` - Set timestamp
- `Build() *Event` - Build final event (unsigned)

#### Specific Tag Methods
- `PTag(pubkey string, relayHint ...string) *EventBuilder` - Add pubkey reference
- `ETag(eventID string, relayHint, marker string) *EventBuilder` - Add event reference  
- `RTag(relayURL string, marker string) *EventBuilder` - Add relay reference
- `DTag(identifier string) *EventBuilder` - Add identifier tag
- `ATag(kind int, pubkey string, dTag string, relayHint ...string) *EventBuilder` - Add address reference
- `TTag(hashtag string) *EventBuilder` - Add hashtag

### Key Management Functions

- `GenerateKeyPair() (*KeyPair, error)` - Generate new random key pair
- `DerivePubkey(privkey string) (string, error)` - Derive public key from private key
- `EncodePrivkey(privkey string) (string, error)` - Convert private key to nsec format
- `EncodePubkey(pubkey string) (string, error)` - Convert public key to npub format
- `DecodeNsec(nsec string) (string, error)` - Convert nsec to private key hex
- `DecodeNpub(npub string) (string, error)` - Convert npub to public key hex

## Examples

### Complete Nostr Client Example

```go
package main

import (
    "log"
    "time"
    "context"
    "github.com/0ceanslim/grain/client/core"
    "github.com/0ceanslim/grain/client/core/tools"
    nostr "github.com/0ceanslim/grain/server/types"
)

func main() {
    // 1. Generate or load keys
    keyPair, err := tools.GenerateKeyPair()
    if err != nil {
        log.Fatal("Key generation failed:", err)
    }
    
    log.Printf("Generated identity: %s", keyPair.Npub)
    
    // 2. Create and configure client
    config := core.DefaultConfig()
    config.DefaultRelays = []string{
        "wss://relay.damus.io",
        "wss://nos.lol",
        "wss://relay.nostr.band",
    }
    
    client := core.NewClient(config)
    defer client.Close()
    
    // 3. Connect to relays
    if err := client.ConnectToRelaysWithRetry(config.DefaultRelays, 3); err != nil {
        log.Printf("Some relays failed to connect: %v", err)
    }
    
    connectedCount := len(client.GetConnectedRelays())
    log.Printf("Connected to %d relays", connectedCount)
    
    if connectedCount == 0 {
        log.Fatal("No relay connections available")
    }
    
    // 4. Create signer for publishing
    signer, err := core.NewPrivateKeySigner(keyPair.PrivateKey)
    if err != nil {
        log.Fatal("Failed to create signer:", err)
    }
    
    // 5. Subscribe to global feed
    filters := []nostr.Filter{
        {
            Kinds: []int{1}, // Text notes only
            Limit: &[]int{20}[0],
        },
    }
    
    sub, err := client.Subscribe(filters, nil)
    if err != nil {
        log.Fatal("Subscription failed:", err)
    }
    
    // 6. Process events for a bit
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    defer sub.Close()
    
    eventCount := 0
    
    go func() {
        for {
            select {
            case event := <-sub.Events:
                eventCount++
                log.Printf("Event #%d: %s (kind %d) by %s", 
                    eventCount, event.ID[:8], event.Kind, event.PubKey[:8])
                    
                if len(event.Content) > 0 {
                    content := event.Content
                    if len(content) > 100 {
                        content = content[:100] + "..."
                    }
                    log.Printf("  Content: %s", content)
                }
                
            case relayURL := <-sub.EOSE:
                log.Printf("EOSE from %s", relayURL)
                
            case err := <-sub.Errors:
                log.Printf("Subscription error: %v", err)
                
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // 7. Publish a note after collecting some events
    time.Sleep(5 * time.Second)
    
    noteContent := "Hello Nostr! This is my first note from the Grain library 🚀"
    note := core.NewTextNote(noteContent)
        .TTag("hello")
        .TTag("grain")
        .TTag("nostr")
    
    event, results, err := core.PublishEvent(client, signer, note, nil)
    if err != nil {
        log.Printf("Failed to publish note: %v", err)
    } else {
        log.Printf("Published note: %s", event.ID)
        
        summary := core.SummarizeBroadcast(results)
        log.Printf("Broadcast: %d/%d relays (%.1f%% success)", 
            summary.Successful, summary.TotalRelays, summary.SuccessRate)
    }
    
    // 8. Wait for subscription to complete
    <-ctx.Done()
    log.Printf("Processed %d events total", eventCount)
}
```

### Web API Integration Example

```go
package main

import (
    "net/http"
    "log"
    "github.com/0ceanslim/grain/client"
    "github.com/0ceanslim/grain/client/api"
)

func main() {
    // Initialize client components
    if err := client.Init(); err != nil {
        log.Fatal("Client init failed:", err)
    }
    
    // Register API endpoints
    client.RegisterEndpoints()
    
    // Add custom routes if needed
    http.HandleFunc("/api/custom", func(w http.ResponseWriter, r *http.Request) {
        // Your custom API logic here
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok"}`))
    })
    
    log.Println("Starting server on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Available API endpoints:
- `POST /api/login` - User authentication
- `GET /api/session` - Get current session
- `POST /api/logout` - End session
- `GET /api/keys/generate` - Generate new key pair
- `GET /api/events` - Query events
- `POST /api/connect` - Connect to relays  
- `POST /api/disconnect` - Disconnect from relays
- `GET /api/relays` - Get relay status

### Subscription Patterns

```go
// Pattern 1: Real-time mentions
func watchMentions(client *core.Client, userPubkey string) {
    filters := []nostr.Filter{
        {
            Kinds: []int{1}, // Text notes
            Tags: map[string][]string{
                "p": {userPubkey}, // Mentions this user
            },
        },
    }
    
    sub, err := client.Subscribe(filters, nil)
    if err != nil {
        log.Printf("Mention subscription failed: %v", err)
        return
    }
    defer sub.Close()
    
    for event := range sub.Events {
        log.Printf("You were mentioned by %s: %s", event.PubKey[:8], event.Content)
    }
}

// Pattern 2: Timeline for following list
func watchTimeline(client *core.Client, followingPubkeys []string) {
    filters := []nostr.Filter{
        {
            Authors: followingPubkeys,
            Kinds:   []int{1, 6}, // Notes and reposts
            Limit:   &[]int{100}[0],
        },
    }
    
    sub, err := client.Subscribe(filters, nil)
    if err != nil {
        log.Printf("Timeline subscription failed: %v", err)
        return
    }
    defer sub.Close()
    
    for event := range sub.Events {
        log.Printf("Timeline: %s posted: %s", event.PubKey[:8], 
            event.Content[:min(50, len(event.Content))])
    }
}

// Pattern 3: Event thread
func watchThread(client *core.Client, rootEventID string) {
    filters := []nostr.Filter{
        {
            Tags: map[string][]string{
                "e": {rootEventID}, // References this event
            },
            Kinds: []int{1, 7}, // Notes and reactions
        },
    }
    
    sub, err := client.Subscribe(filters, nil)
    if err != nil {
        log.Printf("Thread subscription failed: %v", err)
        return
    }
    defer sub.Close()
    
    for event := range sub.Events {
        if event.Kind == 1 {
            log.Printf("Reply: %s", event.Content)
        } else if event.Kind == 7 {
            log.Printf("Reaction: %s", event.Content)
        }
    }
}
```

This comprehensive guide covers all the essential aspects of using Grain as a Nostr client library. The library provides both low-level control for advanced use cases and high-level convenience methods for common operations.