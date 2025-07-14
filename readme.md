# GRAIN 🌾

[![Go Version](https://img.shields.io/github/go-mod/go-version/0ceanslim/grain)](https://golang.org/)
[![GitHub release](https://img.shields.io/github/v/release/0ceanslim/grain)](https://github.com/0ceanslim/grain/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/0ceanslim/grain.svg)](https://pkg.go.dev/github.com/0ceanslim/grain)
[![GitHub downloads](https://img.shields.io/github/downloads/0ceanslim/grain/total)](https://github.com/0ceanslim/grain/releases)
[![GitHub stars](https://img.shields.io/github/stars/0ceanslim/grain?style=social)](https://github.com/0ceanslim/grain/stargazers)

## Go Relay Architecture for Implementing Nostr

GRAIN is a comprehensive Nostr solution that serves primarily as a powerful relay for operators who need fine-grained control, while also providing a complete Go client library for developers building Nostr applications.

## What is Nostr?

Nostr is a simple, open protocol for creating censorship-resistant social networks. Users publish signed events (posts, profiles, reactions) to relays, which store and distribute them. Unlike centralized platforms, users control their identity through cryptographic keys and can freely move between relays.

GRAIN acts as one of these relays - storing events, serving them to clients, and ensuring your relay operates according to your policies. It also provides the building blocks for creating your own Nostr clients.

## Why GRAIN?

### **Powerful Relay Engine**

#### **Intelligent Content Control**

- Real-time blacklist/whitelist filtering with automatic caching
- Word-based content filtering that escalates temporary bans to permanent ones
- Import blacklists from Nostr mute lists (kind 10000 events)
- Domain-based whitelisting by fetching pubkeys from `.well-known/nostr.json`

#### **Intelligent Management**

- Hot configuration reloading - change settings without restarting
- Comprehensive structured logging with automatic rotation
- Memory-aware connection management prevents resource exhaustion
- Multi-layer rate limiting (connections, events, queries) with per-kind controls

#### **Event Management**

- Supports all Nostr event categories: regular, replaceable, addressable, ephemeral & deletion events
- Automatic event deletion handling (kind 5 events) with proper cascade cleanup
- Intelligent event purging with category-based retention policies
- MongoDB storage optimized for Nostr's event structure

#### **Performance Focused**

- Unified cross-collection database queries for efficient event retrieval
- Per-kind MongoDB collections with automatic indexing
- Configurable event size limits to prevent abuse
- Connection pooling and timeout management

### **Complete Go Client Library**

GRAIN now includes a full-featured Nostr client library that developers can use to build their own applications:

#### **Core Client Features**

- **Connection pooling** with automatic relay management
- **Event publishing** with multi-relay broadcasting and result aggregation
- **Subscription management** with filter support and relay hints
- **Event signing** with private key support and extensible signer interface
- **Session management** with user authentication and session persistence

#### **Production-Ready Components**

- **Structured logging** integration matching relay standards
- **Error handling** with proper context and type safety
- **Concurrent operations** with proper synchronization
- **Memory management** with buffered channels and cleanup routines

#### **Developer Experience**

- **Standard library first** - minimal dependencies
- **Clear documentation** with examples and best practices
- **Go modules support** for easy integration
- **Consistent API** following Go conventions

## Web Interface & Reference Implementation

GRAIN includes a modern web interface that serves as both a relay dashboard and reference client implementation:

### **Relay Dashboard**

- **Real-time monitoring** of relay status, connections, and event flow
- **Configuration management** with hot-reload support for all settings
- **User management** with whitelist/blacklist administration
- **Visual analytics** showing relay performance and usage patterns

### **Reference Client**

The web interface showcases the client library capabilities with:

- **User authentication** supporting multiple signing methods
- **Event publishing** with multi-relay support and status tracking
- **Profile management** with metadata caching and display
- **Session handling** demonstrating secure user flows

This reference implementation serves as both a functional interface and documentation for developers using the client library.

### **API Endpoints**

- **RESTful APIs** for client integration and relay management
- **NIP-11 compliance** for relay discovery and metadata
- **Progressive Web App** features for mobile-friendly usage

## 🌾 Wheat Relay Status
[![Wheat Status](https://img.shields.io/badge/dynamic/yaml?url=https%3A%2F%2Fraw.githubusercontent.com%2F0ceanSlim%2Fupptime%2FHEAD%2Fhistory%2Fwheat.yml&query=%24.status&label=Status&logo=statuspage)](https://0ceanSlim.github.io/upptime/history/wheat)
[![24h Uptime](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2F0ceanSlim%2Fupptime%2FHEAD%2Fapi%2Fwheat%2Fuptime-day.json)](https://0ceanSlim.github.io/upptime/history/wheat)
[![Overall Uptime](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2F0ceanSlim%2Fupptime%2FHEAD%2Fapi%2Fwheat%2Fuptime.json)](https://0ceanSlim.github.io/upptime/history/wheat)
[![Response Time](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2F0ceanSlim%2Fupptime%2FHEAD%2Fapi%2Fwheat%2Fresponse-time.json)](https://0ceanSlim.github.io/upptime/history/wheat)

**Development Relay**: `wss://wheat.happytavern.co`

[📊 **View Detailed Status & Historical Data**](https://0ceanSlim.github.io/upptime/history/wheat)

My development relay **wheat.happytavern.co** serves as the testing and demo environment for Grain. This relay helps me validate new features, test performance optimizations, and provide a platform for developers to experiment with grain. This relay routinely runs unreleased versions of grain and may contain bugs.

Wheat is a public nostr relay that anyone can write to and read from. Wheat will delete events from non whitelisted users periodically. You can add your npub to the whitelist by paying for a [Happy Tavern NIP05](https://happytavern.co/nostr-verified).

*Status monitoring powered by [Upptime](https://github.com/upptime/upptime)*

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

## Using GRAIN as a Go Library

GRAIN can be imported as a Go module for building Nostr clients:

```go
package main

import (
    "fmt"
    "github.com/0ceanslim/grain/client/core"
    nostr "github.com/0ceanslim/grain/server/types"
)

func main() {
    // Create client with default configuration
    client := core.NewClient(nil)

    // Connect to relays
    relays := []string{"wss://relay.damus.io", "wss://nos.lol"}
    if err := client.ConnectToRelays(relays); err != nil {
        panic(err)
    }

    // Create and sign an event
    signer, _ := core.NewEventSigner("your-private-key-hex")
    event := signer.CreateEvent(1, "Hello Nostr!", nil)

    // Broadcast to all connected relays
    results := client.BroadcastEvent(event, nil)
    fmt.Printf("Broadcast results: %+v\n", results)

    // Subscribe to events
    filters := []nostr.Filter{{Kinds: []int{1}, Limit: 10}}
    sub, _ := client.Subscribe(filters, nil)

    // Handle incoming events
    go func() {
        for event := range sub.Events {
            fmt.Printf("Received event: %s\n", event.Content)
        }
    }()
}
```

> **Note**: A comprehensive Client Library Guide with complete examples and documentation will be available soon after all client methods are fully implemented.

## License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.

## Contributing

I welcome contributions, bug reports, and feature requests via GitHub.

**Repository**: <https://github.com/0ceanslim/grain>  
**Issues**: <https://github.com/0ceanslim/grain/issues>

### Development Resources

- **🔧 Development Guide** - _[Development Documentation](docs/development/readme.md)_
- **🧪 Testing Guide** - _[Testing Documentation](tests/readme.md)_
- **📚 API Documentation** - _[API Documentation](docs/api.md)_

These guides cover setting up your development environment, code standards, testing procedures, client library usage, and contribution workflows.

---

made with 💦 by [OceanSlim](https://njump.me/npub1zmc6qyqdfnllhnzzxr5wpepfpnzcf8q6m3jdveflmgruqvd3qa9sjv7f60)

_Reliable infrastructure for the decentralized web._
