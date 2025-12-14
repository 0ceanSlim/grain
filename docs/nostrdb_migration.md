# MongoDB to nostrdb Migration - Comprehensive Guide

## Table of Contents

1. [Overview](#overview)
2. [Architecture & Design](#architecture--design)
3. [Build System Integration](#build-system-integration)
4. [Database Abstraction Layer](#database-abstraction-layer)
5. [CGo Integration](#cgo-integration)
6. [nostrdb Implementation](#nostrdb-implementation)
7. [Configuration](#configuration)
8. [Testing Strategy](#testing-strategy)
9. [Implementation Phases](#implementation-phases)
10. [Critical Technical Challenges](#critical-technical-challenges)
11. [File Modifications](#file-modifications)
12. [Performance Tuning](#performance-tuning)
13. [Troubleshooting](#troubleshooting)
14. [Data Migration](#data-migration)

---

## Overview

### Goal

Replace MongoDB with nostrdb (C library with LMDB backend by William Casarin) while maintaining MongoDB as a deprecated alternative. Implementation uses CGo, feature flags, and a database abstraction layer.

### Current State

- **Database**: MongoDB exclusively (`go.mongodb.org/mongo-driver v1.16.0`)
- **Architecture**: Go 1.23+ Nostr relay, no database abstraction
- **Database Code**: ~1,297 lines in `/server/db/mongo/`
- **Storage Model**: Per-kind collections (event-kind0, event-kind1, etc.)
- **Operations**: CRUD, complex aggregation queries, tag filtering

### User Requirements

1. Integrate nostrdb C library (ndb.c with LMDB backend)
2. Feature flag to switch between MongoDB and nostrdb
3. Get nostrdb working first, migrate data later
4. Keep MongoDB as deprecated option
5. Use CGo for C library integration

---

## Architecture & Design

### Database Abstraction Interface

All database backends implement a common interface:

```go
// File: /home/satoshi/grain/server/db/interface.go

package db

import (
    "context"
    nostr "github.com/0ceanslim/grain/server/types"
)

// Database defines the interface that all database backends must implement
type Database interface {
    // Connection Management
    Init(config interface{}) error
    Close() error
    IsHealthy(ctx context.Context) bool

    // Event Storage Operations
    StoreEvent(ctx context.Context, evt nostr.Event, client nostr.ClientInterface) error
    CheckDuplicate(ctx context.Context, evt nostr.Event) (bool, error)

    // Event Query Operations
    QueryEvents(ctx context.Context, filters []nostr.Filter) ([]nostr.Event, error)

    // Event Deletion
    DeleteEvents(ctx context.Context, filter interface{}) error

    // Maintenance
    PurgeOldEvents(ctx context.Context, config interface{}) error
    FetchAllUsers(ctx context.Context) ([]string, error)

    // Database-specific operations (for advanced features)
    GetNativeClient() interface{} // Returns *mongo.Client or *ndb.DB
}
```

### Backend Selection Logic

```go
// File: /home/satoshi/grain/server/db/factory.go

package db

import (
    "fmt"
    cfgType "github.com/0ceanslim/grain/config/types"
)

type DatabaseType string

const (
    DatabaseTypeMongoDB DatabaseType = "mongodb"
    DatabaseTypeNostrDB DatabaseType = "nostrdb"
)

func NewDatabase(cfg *cfgType.ServerConfig) (Database, error) {
    switch cfg.Database.Type {
    case DatabaseTypeMongoDB:
        return newMongoDatabase(cfg)
    case DatabaseTypeNostrDB:
        return newNostrDBDatabase(cfg)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
    }
}
```

### Package Structure

```
/server/db/
├── interface.go           # Database interface definition
├── factory.go             # Backend selection factory
├── mongo_adapter.go       # MongoDB adapter (wraps existing mongo package)
├── mongo/                 # Existing MongoDB implementation
│   ├── dbMongo.go
│   ├── storeMongo.go
│   ├── queryMongo.go
│   └── eventStore/
│       ├── regular.go
│       ├── replaceable.go
│       ├── addressable.go
│       └── delete.go
└── nostrdb/               # New nostrdb implementation
    ├── dbNostrdb.go       # Init, connection, health
    ├── storeNostrdb.go    # Event routing
    ├── queryNostrdb.go    # Query implementation
    ├── checkDuplicate.go  # Duplicate detection
    ├── purgeEvents.go     # Event purging
    ├── fetchAllUsers.go   # User enumeration
    ├── cgo_wrapper.go     # CGo interop layer
    ├── types.go           # Type definitions
    ├── transactions.go    # Transaction helpers
    └── eventStore/
        ├── regular.go
        ├── replaceable.go
        ├── addressable.go
        ├── delete.go
        ├── deprecated.go
        └── unknown.go
```

---

## Build System Integration

### Dependencies Installation

#### Linux (Debian/Ubuntu)
```bash
sudo apt-get update
sudo apt-get install -y liblmdb-dev build-essential
```

#### macOS
```bash
brew install lmdb
```

### Adding nostrdb Submodule

```bash
cd /home/satoshi/grain
git submodule add https://github.com/damus-io/nostrdb vendor/nostrdb
git submodule update --init --recursive
```

### Build Process

Since tests already have a Makefile, we'll integrate the nostrdb build into the existing build flow:

```bash
# Build nostrdb C library
cd vendor/nostrdb
make

# Build GRAIN with nostrdb support
cd /home/satoshi/grain
go build -tags nostrdb
```

### CGo Build Flags

Add to the top of nostrdb Go files (e.g., `dbNostrdb.go`):

```go
/*
#cgo CFLAGS: -I${SRCDIR}/../../vendor/nostrdb/src
#cgo LDFLAGS: -L${SRCDIR}/../../vendor/nostrdb/build -lnostrdb -llmdb
#include "nostrdb.h"
*/
import "C"
```

---

## Database Abstraction Layer

### MongoDB Adapter Implementation

Wrap existing MongoDB package to implement the Database interface:

```go
// File: /home/satoshi/grain/server/db/mongo_adapter.go

package db

import (
    "context"
    "github.com/0ceanslim/grain/server/db/mongo"
    nostr "github.com/0ceanslim/grain/server/types"
    cfgType "github.com/0ceanslim/grain/config/types"
)

type mongoAdapter struct {
    cfg *cfgType.ServerConfig
}

func newMongoDatabase(cfg *cfgType.ServerConfig) (Database, error) {
    return &mongoAdapter{cfg: cfg}, nil
}

func (m *mongoAdapter) Init(config interface{}) error {
    return mongo.InitializeDatabase(m.cfg)
}

func (m *mongoAdapter) Close() error {
    return mongo.DisconnectDB()
}

func (m *mongoAdapter) IsHealthy(ctx context.Context) bool {
    return mongo.IsClientHealthy(ctx)
}

func (m *mongoAdapter) StoreEvent(ctx context.Context, evt nostr.Event, client nostr.ClientInterface) error {
    return mongo.StoreMongoEvent(ctx, evt, client)
}

func (m *mongoAdapter) CheckDuplicate(ctx context.Context, evt nostr.Event) (bool, error) {
    return mongo.CheckDuplicateEvent(ctx, evt)
}

func (m *mongoAdapter) QueryEvents(ctx context.Context, filters []nostr.Filter) ([]nostr.Event, error) {
    return mongo.QueryEvents(ctx, filters)
}

func (m *mongoAdapter) DeleteEvents(ctx context.Context, filter interface{}) error {
    // Implement based on existing mongo delete operations
    return nil
}

func (m *mongoAdapter) PurgeOldEvents(ctx context.Context, config interface{}) error {
    // Use existing purge logic
    return nil
}

func (m *mongoAdapter) FetchAllUsers(ctx context.Context) ([]string, error) {
    return mongo.GetAllAuthorsFromRelay(ctx)
}

func (m *mongoAdapter) GetNativeClient() interface{} {
    return mongo.GetClient()
}
```

---

## CGo Integration

### Type Conversion Layer

```go
// File: /home/satoshi/grain/server/db/nostrdb/cgo_wrapper.go

package nostrdb

/*
#cgo CFLAGS: -I${SRCDIR}/../../vendor/nostrdb/src
#cgo LDFLAGS: -L${SRCDIR}/../../vendor/nostrdb/build -lnostrdb -llmdb
#include <stdlib.h>
#include "nostrdb.h"
*/
import "C"
import (
    "errors"
    "unsafe"
    nostr "github.com/0ceanslim/grain/server/types"
)

// Convert Go Event to C ndb_note
func goEventToCNote(evt nostr.Event) (*C.struct_ndb_note, error) {
    // IMPORTANT: All C.CString allocations must be freed by caller

    cID := C.CString(evt.ID)
    defer C.free(unsafe.Pointer(cID))

    cPubkey := C.CString(evt.PubKey)
    defer C.free(unsafe.Pointer(cPubkey))

    cContent := C.CString(evt.Content)
    defer C.free(unsafe.Pointer(cContent))

    cSig := C.CString(evt.Sig)
    defer C.free(unsafe.Pointer(cSig))

    // Convert tags array (complex marshaling)
    // This is pseudo-code - actual implementation depends on nostrdb API
    var cTags *C.char
    if len(evt.Tags) > 0 {
        // Serialize tags to JSON or nostrdb format
        // tagsJSON := marshalTagsForC(evt.Tags)
        // cTags = C.CString(tagsJSON)
        // defer C.free(unsafe.Pointer(cTags))
    }

    // Create ndb_note structure
    // Actual struct depends on nostrdb.h definition
    // This is illustrative - check actual API
    note := C.ndb_note_create(
        cID,
        cPubkey,
        C.int(evt.Kind),
        C.longlong(evt.CreatedAt),
        cContent,
        cSig,
        cTags,
    )

    return note, nil
}

// Convert C ndb_note to Go Event
func cNoteToGoEvent(note *C.struct_ndb_note) (nostr.Event, error) {
    if note == nil {
        return nostr.Event{}, errors.New("nil note")
    }

    // Extract fields from C struct
    // Actual field access depends on nostrdb.h definition
    evt := nostr.Event{
        ID:        C.GoString(note.id),
        PubKey:    C.GoString(note.pubkey),
        Kind:      int(note.kind),
        CreatedAt: int64(note.created_at),
        Content:   C.GoString(note.content),
        Sig:       C.GoString(note.sig),
    }

    // Parse tags from C format
    // evt.Tags = parseTagsFromC(note.tags)

    return evt, nil
}

// Safe C string allocation (caller must free)
func allocCString(s string) *C.char {
    return C.CString(s)
}

// Convert C error code to Go error
func cErrorToGoError(cErr C.int) error {
    if cErr == 0 {
        return nil
    }
    // Map C error codes to meaningful errors
    return errors.New("nostrdb error: " + string(rune(cErr)))
}
```

### Memory Management Rules

**CRITICAL SAFETY RULES:**

1. **Every `C.CString()` must have `defer C.free()`**
   ```go
   cStr := C.CString(goStr)
   defer C.free(unsafe.Pointer(cStr))
   ```

2. **Always check for nil before dereferencing C pointers**
   ```go
   if cPtr == nil {
       return errors.New("null pointer")
   }
   ```

3. **Use defer for cleanup even in error paths**
   ```go
   func someFunc() error {
       cStr := C.CString("test")
       defer C.free(unsafe.Pointer(cStr))

       if err := someCCall(); err != nil {
           return err // defer still runs!
       }
       return nil
   }
   ```

4. **Don't store C pointers in Go structs**
   ```go
   // BAD
   type BadStruct struct {
       cPtr *C.char // Will cause garbage collection issues
   }

   // GOOD
   type GoodStruct struct {
       goStr string // Convert to Go string immediately
   }
   ```

---

## nostrdb Implementation

### Database Initialization

```go
// File: /home/satoshi/grain/server/db/nostrdb/dbNostrdb.go

package nostrdb

/*
#cgo CFLAGS: -I${SRCDIR}/../../vendor/nostrdb/src
#cgo LDFLAGS: -L${SRCDIR}/../../vendor/nostrdb/build -lnostrdb -llmdb
#include "nostrdb.h"
*/
import "C"
import (
    "context"
    "errors"
    "unsafe"
    cfgType "github.com/0ceanslim/grain/config/types"
)

type NostrDB struct {
    db   *C.struct_ndb
    cfg  *cfgType.NostrDBConfig
    path string
}

func InitDB(cfg *cfgType.NostrDBConfig) (*NostrDB, error) {
    if cfg.Path == "" {
        return nil, errors.New("nostrdb path not configured")
    }

    cPath := C.CString(cfg.Path)
    defer C.free(unsafe.Pointer(cPath))

    // Initialize LMDB with configured settings
    var cdb *C.struct_ndb

    // Call nostrdb initialization (check actual API)
    // Pseudo-code based on typical LMDB/nostrdb patterns:
    ret := C.ndb_init(
        &cdb,
        cPath,
        C.size_t(cfg.MapSize),
        C.int(cfg.MaxDatabases),
        C.int(cfg.MaxReaders),
    )

    if ret != 0 {
        return nil, errors.New("failed to initialize nostrdb")
    }

    return &NostrDB{
        db:   cdb,
        cfg:  cfg,
        path: cfg.Path,
    }, nil
}

func (ndb *NostrDB) Close() error {
    if ndb.db != nil {
        C.ndb_close(ndb.db)
        ndb.db = nil
    }
    return nil
}

func (ndb *NostrDB) IsHealthy(ctx context.Context) bool {
    if ndb.db == nil {
        return false
    }

    // Perform simple read operation to test health
    // Implementation depends on nostrdb API
    ret := C.ndb_ping(ndb.db)
    return ret == 0
}

// Transaction wrapper for atomicity
func (ndb *NostrDB) withTransaction(fn func(*C.struct_ndb_txn) error) error {
    if ndb.db == nil {
        return errors.New("database not initialized")
    }

    var txn *C.struct_ndb_txn
    ret := C.ndb_begin_transaction(ndb.db, &txn, 0) // 0 = read-write
    if ret != 0 {
        return errors.New("failed to begin transaction")
    }
    defer C.ndb_txn_abort(txn) // Abort if not committed

    if err := fn(txn); err != nil {
        return err
    }

    ret = C.ndb_txn_commit(txn)
    if ret != 0 {
        return errors.New("failed to commit transaction")
    }

    return nil
}
```

### Event Storage

```go
// File: /home/satoshi/grain/server/db/nostrdb/storeNostrdb.go

package nostrdb

import (
    "context"
    nostr "github.com/0ceanslim/grain/server/types"
    "github.com/0ceanslim/grain/server/db/nostrdb/eventStore"
)

func (ndb *NostrDB) StoreEvent(ctx context.Context, evt nostr.Event, client nostr.ClientInterface) error {
    // Route to appropriate handler based on event kind
    switch {
    case evt.Kind == 0 || evt.Kind == 3 || (evt.Kind >= 10000 && evt.Kind < 20000):
        // Replaceable events
        return eventStore.Replaceable(ctx, ndb, evt, client)

    case evt.Kind >= 30000 && evt.Kind < 40000:
        // Addressable/parameterized replaceable events
        return eventStore.Addressable(ctx, ndb, evt, client)

    case evt.Kind == 5:
        // Deletion events
        return eventStore.Delete(ctx, ndb, evt, client)

    case evt.Kind >= 20000 && evt.Kind < 30000:
        // Ephemeral events (don't store)
        return eventStore.Ephemeral(ctx, ndb, evt, client)

    default:
        // Regular events
        return eventStore.Regular(ctx, ndb, evt, client)
    }
}
```

### Regular Event Handler

```go
// File: /home/satoshi/grain/server/db/nostrdb/eventStore/regular.go

package eventStore

/*
#cgo CFLAGS: -I${SRCDIR}/../../../vendor/nostrdb/src
#cgo LDFLAGS: -L${SRCDIR}/../../../vendor/nostrdb/build -lnostrdb -llmdb
#include "nostrdb.h"
*/
import "C"
import (
    "context"
    nostr "github.com/0ceanslim/grain/server/types"
)

func Regular(ctx context.Context, ndb interface{}, evt nostr.Event, client nostr.ClientInterface) error {
    // Convert Go event to C note
    cNote, err := goEventToCNote(evt)
    if err != nil {
        return err
    }
    defer C.ndb_note_free(cNote)

    // Write to nostrdb
    db := ndb.(*NostrDB).db
    ret := C.ndb_write_note(db, cNote)

    if ret != 0 {
        return cErrorToGoError(ret)
    }

    return nil
}
```

### Replaceable Event Handler

```go
// File: /home/satoshi/grain/server/db/nostrdb/eventStore/replaceable.go

package eventStore

import (
    "context"
    nostr "github.com/0ceanslim/grain/server/types"
)

func Replaceable(ctx context.Context, ndb interface{}, evt nostr.Event, client nostr.ClientInterface) error {
    db := ndb.(*NostrDB)

    // Transaction ensures atomicity
    return db.withTransaction(func(txn *C.struct_ndb_txn) error {
        // 1. Query existing event by pubkey + kind
        existing, err := queryReplaceableEvent(txn, evt.PubKey, evt.Kind)

        if err == nil && existing != nil {
            // 2. Compare timestamps (NIP-01 logic)
            if existing.CreatedAt > evt.CreatedAt ||
               (existing.CreatedAt == evt.CreatedAt && existing.ID < evt.ID) {
                return errors.New("newer event exists")
            }

            // 3. Delete old event
            if err := deleteEventByID(txn, existing.ID); err != nil {
                return err
            }
        }

        // 4. Insert new event
        cNote, err := goEventToCNote(evt)
        if err != nil {
            return err
        }
        defer C.ndb_note_free(cNote)

        ret := C.ndb_write_note_txn(txn, cNote)
        return cErrorToGoError(ret)
    })
}

func queryReplaceableEvent(txn *C.struct_ndb_txn, pubkey string, kind int) (*nostr.Event, error) {
    cPubkey := C.CString(pubkey)
    defer C.free(unsafe.Pointer(cPubkey))

    // Query nostrdb for existing replaceable event
    var cNote *C.struct_ndb_note
    ret := C.ndb_query_replaceable(txn, cPubkey, C.int(kind), &cNote)

    if ret != 0 || cNote == nil {
        return nil, errors.New("event not found")
    }

    evt, err := cNoteToGoEvent(cNote)
    return &evt, err
}

func deleteEventByID(txn *C.struct_ndb_txn, id string) error {
    cID := C.CString(id)
    defer C.free(unsafe.Pointer(cID))

    ret := C.ndb_delete_note(txn, cID)
    return cErrorToGoError(ret)
}
```

### Query Implementation

```go
// File: /home/satoshi/grain/server/db/nostrdb/queryNostrdb.go

package nostrdb

import (
    "context"
    nostr "github.com/0ceanslim/grain/server/types"
)

func (ndb *NostrDB) QueryEvents(ctx context.Context, filters []nostr.Filter) ([]nostr.Event, error) {
    var allEvents []nostr.Event

    for _, filter := range filters {
        events, err := ndb.queryWithFilter(ctx, filter)
        if err != nil {
            return nil, err
        }
        allEvents = append(allEvents, events...)
    }

    // Deduplicate and sort by created_at descending
    allEvents = deduplicateEvents(allEvents)
    sortEventsByCreatedAt(allEvents)

    // Apply global limit if specified
    if len(filters) > 0 && filters[0].Limit > 0 {
        limit := filters[0].Limit
        if len(allEvents) > limit {
            allEvents = allEvents[:limit]
        }
    }

    return allEvents, nil
}

func (ndb *NostrDB) queryWithFilter(ctx context.Context, filter nostr.Filter) ([]nostr.Event, error) {
    var events []nostr.Event

    // Build nostrdb query from Nostr filter
    query := ndb.buildNostrDBQuery(filter)

    // Execute query
    // Pseudo-code - actual implementation depends on nostrdb API
    var cResults *C.struct_ndb_query_result
    ret := C.ndb_query(ndb.db, query, &cResults)
    if ret != 0 {
        return nil, cErrorToGoError(ret)
    }
    defer C.ndb_query_result_free(cResults)

    // Convert results to Go events
    count := int(C.ndb_query_result_count(cResults))
    for i := 0; i < count; i++ {
        cNote := C.ndb_query_result_get(cResults, C.int(i))
        evt, err := cNoteToGoEvent(cNote)
        if err != nil {
            continue
        }
        events = append(events, evt)
    }

    return events, nil
}

func (ndb *NostrDB) buildNostrDBQuery(filter nostr.Filter) *C.struct_ndb_query {
    // Build nostrdb query structure from Nostr filter
    // This depends heavily on nostrdb's actual query API

    query := C.ndb_query_create()

    // Add ID filters
    if len(filter.IDs) > 0 {
        for _, id := range filter.IDs {
            cID := C.CString(id)
            C.ndb_query_add_id(query, cID)
            C.free(unsafe.Pointer(cID))
        }
    }

    // Add author filters
    if len(filter.Authors) > 0 {
        for _, author := range filter.Authors {
            cAuthor := C.CString(author)
            C.ndb_query_add_author(query, cAuthor)
            C.free(unsafe.Pointer(cAuthor))
        }
    }

    // Add kind filters
    if len(filter.Kinds) > 0 {
        for _, kind := range filter.Kinds {
            C.ndb_query_add_kind(query, C.int(kind))
        }
    }

    // Add tag filters (complex)
    for tagKey, tagValues := range filter.Tags {
        cTagKey := C.CString(tagKey)
        for _, tagValue := range tagValues {
            cTagValue := C.CString(tagValue)
            C.ndb_query_add_tag(query, cTagKey, cTagValue)
            C.free(unsafe.Pointer(cTagValue))
        }
        C.free(unsafe.Pointer(cTagKey))
    }

    // Add time filters
    if filter.Since > 0 {
        C.ndb_query_set_since(query, C.longlong(filter.Since))
    }
    if filter.Until > 0 {
        C.ndb_query_set_until(query, C.longlong(filter.Until))
    }

    // Add limit
    if filter.Limit > 0 {
        C.ndb_query_set_limit(query, C.int(filter.Limit))
    }

    return query
}
```

---

## Configuration

### Updated Config Structure

```go
// File: /home/satoshi/grain/config/types/serverConfig.go

// Add to existing ServerConfig struct:

type ServerConfig struct {
    // ... existing fields ...

    Database DatabaseConfig `yaml:"database"`

    // DEPRECATED: Kept for backward compatibility
    MongoDB struct {
        URI      string `yaml:"uri"`
        Database string `yaml:"database"`
    } `yaml:"mongodb"`
}

type DatabaseConfig struct {
    Type    string         `yaml:"type"`     // "mongodb" or "nostrdb"
    MongoDB MongoDBConfig  `yaml:"mongodb"`
    NostrDB NostrDBConfig  `yaml:"nostrdb"`
}

type MongoDBConfig struct {
    URI      string `yaml:"uri"`
    Database string `yaml:"database"`
}

type NostrDBConfig struct {
    Path         string `yaml:"path"`
    MapSize      int64  `yaml:"map_size"`       // LMDB map size in bytes
    MaxDatabases int    `yaml:"max_databases"`  // Max LMDB databases
    MaxReaders   int    `yaml:"max_readers"`    // Max concurrent readers
}
```

### Example Configuration

```yaml
# File: /home/satoshi/grain/docs/examples/config.example.yml

# Database Configuration
database:
  type: "nostrdb"  # Options: "mongodb" (deprecated), "nostrdb" (recommended)

  # NostrDB Configuration (recommended)
  nostrdb:
    path: "./data/nostrdb"        # Database directory
    map_size: 10737418240         # 10GB LMDB map size
    max_databases: 128            # Maximum LMDB databases
    max_readers: 126              # Maximum concurrent readers

  # MongoDB Configuration (deprecated but still supported)
  mongodb:
    uri: mongodb://localhost:27017/
    database: grain

# DEPRECATED: Legacy MongoDB config (for backward compatibility)
# Will be removed in future version
# mongodb:
#   uri: mongodb://localhost:27017/
#   database: grain
```

### Backward Compatibility Logic

```go
// In config loading code (config/loadConfig.go):

func LoadConfig(path string) (*ServerConfig, error) {
    cfg := &ServerConfig{}
    // ... load YAML ...

    // Backward compatibility: If new database.type is not set, check legacy mongodb config
    if cfg.Database.Type == "" {
        if cfg.MongoDB.URI != "" {
            log.Warn("Using legacy mongodb config format. Please migrate to database.type config.")
            cfg.Database.Type = "mongodb"
            cfg.Database.MongoDB.URI = cfg.MongoDB.URI
            cfg.Database.MongoDB.Database = cfg.MongoDB.Database
        } else {
            return nil, errors.New("no database configuration found")
        }
    }

    return cfg, nil
}
```

---

## Testing Strategy

### Test Organization

Tests are organized in the existing `/tests` folder structure:

```
/tests/
├── Makefile               # Existing test orchestration
├── helpers.go             # Test utilities
├── db/                    # NEW: Database tests
│   ├── abstraction_test.go      # Test database interface
│   ├── mongodb_test.go          # MongoDB adapter tests
│   └── nostrdb_test.go          # nostrdb implementation tests
├── integration/           # Existing integration tests
│   ├── relay_test.go
│   ├── websocket_test.go
│   ├── api_test.go
│   └── database_switch_test.go  # NEW: Test switching backends
├── review/                # Existing code quality tests
│   └── codeQuality_test.go
└── benchmarks/            # NEW: Performance tests
    ├── mongodb_bench_test.go
    └── nostrdb_bench_test.go
```

### Unit Tests

#### Database Abstraction Test

```go
// File: /tests/db/abstraction_test.go

package db_test

import (
    "context"
    "testing"
    "github.com/0ceanslim/grain/server/db"
    "github.com/0ceanslim/grain/config/types"
)

func TestDatabaseInterface(t *testing.T) {
    tests := []struct {
        name string
        dbType string
    }{
        {"MongoDB", "mongodb"},
        {"NostrDB", "nostrdb"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := &types.ServerConfig{}
            cfg.Database.Type = tt.dbType

            database, err := db.NewDatabase(cfg)
            if err != nil {
                t.Fatalf("Failed to create %s: %v", tt.dbType, err)
            }
            defer database.Close()

            // Test all interface methods
            testDatabaseOperations(t, database)
        })
    }
}

func testDatabaseOperations(t *testing.T, db db.Database) {
    ctx := context.Background()

    // Test Init
    if err := db.Init(nil); err != nil {
        t.Errorf("Init failed: %v", err)
    }

    // Test Health Check
    if !db.IsHealthy(ctx) {
        t.Error("Database not healthy")
    }

    // Test Store/Query/Delete operations
    // ...
}
```

#### CGo Memory Test

```go
// File: /tests/db/nostrdb_test.go

package db_test

import (
    "testing"
    "runtime"
    "github.com/0ceanslim/grain/server/db/nostrdb"
)

func TestCGoMemoryManagement(t *testing.T) {
    // Get initial memory stats
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)

    // Perform many CGo operations
    for i := 0; i < 10000; i++ {
        evt := createTestEvent()
        cNote, err := nostrdb.GoEventToCNote(evt)
        if err != nil {
            t.Fatalf("Conversion failed: %v", err)
        }
        // Free should happen via defer
    }

    // Force GC
    runtime.GC()

    // Check memory didn't grow significantly
    var m2 runtime.MemStats
    runtime.ReadMemStats(&m2)

    growth := m2.Alloc - m1.Alloc
    if growth > 10*1024*1024 { // 10MB threshold
        t.Errorf("Memory leak detected: %d bytes", growth)
    }
}
```

### Integration Tests

```go
// File: /tests/integration/database_switch_test.go

package integration_test

import (
    "testing"
    "context"
)

func TestDatabaseBackendSwitch(t *testing.T) {
    // Test that the same events can be stored and retrieved
    // regardless of backend

    testCases := []string{"mongodb", "nostrdb"}

    for _, backend := range testCases {
        t.Run(backend, func(t *testing.T) {
            // Configure backend
            cfg := loadTestConfig()
            cfg.Database.Type = backend

            // Initialize relay
            relay := startTestRelay(cfg)
            defer relay.Stop()

            // Store test events
            events := generateTestEvents(100)
            for _, evt := range events {
                if err := relay.StoreEvent(context.TODO(), evt); err != nil {
                    t.Errorf("Failed to store event: %v", err)
                }
            }

            // Query events
            filters := createTestFilters()
            results, err := relay.QueryEvents(context.TODO(), filters)
            if err != nil {
                t.Fatalf("Query failed: %v", err)
            }

            // Verify results
            if len(results) != len(events) {
                t.Errorf("Expected %d events, got %d", len(events), len(results))
            }
        })
    }
}
```

### Benchmark Tests

```go
// File: /tests/benchmarks/mongodb_bench_test.go

package benchmarks_test

import (
    "testing"
    "context"
)

func BenchmarkMongoDBInsert(b *testing.B) {
    db := setupMongoDB(b)
    defer db.Close()

    events := generateTestEvents(b.N)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        db.StoreEvent(context.TODO(), events[i], nil)
    }
}

func BenchmarkMongoDBQuery(b *testing.B) {
    db := setupMongoDB(b)
    defer db.Close()

    // Pre-populate database
    populateDatabase(db, 10000)

    filters := createQueryFilters()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        db.QueryEvents(context.TODO(), filters)
    }
}
```

```go
// File: /tests/benchmarks/nostrdb_bench_test.go

package benchmarks_test

import (
    "testing"
    "context"
)

func BenchmarkNostrDBInsert(b *testing.B) {
    db := setupNostrDB(b)
    defer db.Close()

    events := generateTestEvents(b.N)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        db.StoreEvent(context.TODO(), events[i], nil)
    }
}

func BenchmarkNostrDBQuery(b *testing.B) {
    db := setupNostrDB(b)
    defer db.Close()

    // Pre-populate database
    populateDatabase(db, 10000)

    filters := createQueryFilters()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        db.QueryEvents(context.TODO(), filters)
    }
}
```

### Running Tests

```bash
# Navigate to tests directory
cd /home/satoshi/grain/tests

# Run all tests with MongoDB
DATABASE_TYPE=mongodb make test

# Run all tests with nostrdb
DATABASE_TYPE=nostrdb make test

# Run benchmarks
cd /home/satoshi/grain
go test -bench=. -benchmem ./tests/benchmarks/

# Run with race detector
go test -race ./tests/db/

# Memory profiling
go test -memprofile=mem.out ./tests/db/
go tool pprof mem.out
```

---

## Implementation Phases

### Phase 1: Foundation & Abstraction (2 weeks)

**Objective**: Create database abstraction layer and set up CGo build system

**Tasks**:
1. ✅ Add nostrdb as git submodule
2. ✅ Install LMDB dependencies
3. ✅ Create `/server/db/interface.go` - Database interface
4. ✅ Create `/server/db/factory.go` - Backend selection
5. ✅ Create `/server/db/mongo_adapter.go` - Wrap existing MongoDB code
6. ✅ Update `/config/types/serverConfig.go` - Add database config
7. ✅ Create tests in `/tests/db/abstraction_test.go`
8. ✅ Test MongoDB adapter (ensure no regression)

**Success Criteria**:
- MongoDB adapter passes all existing tests
- No regression in MongoDB functionality
- Configuration supports both backends

**Files Created**:
- `/server/db/interface.go`
- `/server/db/factory.go`
- `/server/db/mongo_adapter.go`
- `/tests/db/abstraction_test.go`

**Files Modified**:
- `/config/types/serverConfig.go`
- `/docs/examples/config.example.yml`

---

### Phase 2: CGo Wrapper Development (2 weeks)

**Objective**: Implement C interop layer with proper memory management

**Tasks**:
1. ✅ Build nostrdb C library
2. ✅ Create `/server/db/nostrdb/cgo_wrapper.go`
3. ✅ Implement Go Event → C ndb_note conversion
4. ✅ Implement C ndb_note → Go Event conversion
5. ✅ Create memory management helpers
6. ✅ Implement error handling from C calls
7. ✅ Create `/tests/db/nostrdb_memory_test.go`
8. ✅ Test with race detector and memory profiler

**Success Criteria**:
- All CGo calls properly handle memory (no leaks)
- Type conversions preserve data integrity
- Error handling captures all C error cases
- Memory tests pass with -race flag

**Files Created**:
- `/server/db/nostrdb/cgo_wrapper.go`
- `/server/db/nostrdb/types.go`
- `/tests/db/nostrdb_memory_test.go`

---

### Phase 3: Core nostrdb Operations (3 weeks)

**Objective**: Implement event storage operations for all event types

**Tasks**:
1. ✅ Create `/server/db/nostrdb/dbNostrdb.go` - Init, connection, health
2. ✅ Create `/server/db/nostrdb/storeNostrdb.go` - Event routing
3. ✅ Create `/server/db/nostrdb/checkDuplicate.go` - Duplicate detection
4. ✅ Implement event store handlers:
   - `/server/db/nostrdb/eventStore/regular.go`
   - `/server/db/nostrdb/eventStore/replaceable.go`
   - `/server/db/nostrdb/eventStore/addressable.go`
   - `/server/db/nostrdb/eventStore/delete.go`
   - `/server/db/nostrdb/eventStore/deprecated.go`
   - `/server/db/nostrdb/eventStore/unknown.go`
5. ✅ Create `/server/db/nostrdb/transactions.go` - Transaction helpers
6. ✅ Create integration tests in `/tests/integration/nostrdb_storage_test.go`

**Success Criteria**:
- All event types can be stored
- Duplicate detection works correctly
- Replaceable events properly replace older versions
- Addressable events handle d-tags correctly
- Transactions ensure atomicity

**Files Created**:
- `/server/db/nostrdb/dbNostrdb.go`
- `/server/db/nostrdb/storeNostrdb.go`
- `/server/db/nostrdb/checkDuplicate.go`
- `/server/db/nostrdb/transactions.go`
- `/server/db/nostrdb/eventStore/*.go` (6 files)
- `/tests/integration/nostrdb_storage_test.go`

---

### Phase 4: Query Implementation (3 weeks)

**Objective**: Implement Nostr filter queries with proper translation

**Tasks**:
1. ✅ Create `/server/db/nostrdb/queryNostrdb.go`
2. ✅ Implement filter translation (IDs, Authors, Kinds, Tags, Time, Limit)
3. ✅ Implement result sorting (created_at descending)
4. ✅ Handle OR logic (multiple filters)
5. ✅ Optimize tag filtering
6. ✅ Create `/tests/db/nostrdb_query_test.go`
7. ✅ Test all filter combinations
8. ✅ Verify query results match MongoDB output

**Success Criteria**:
- All filter combinations work correctly
- Query results match MongoDB output
- Performance meets or exceeds MongoDB
- Complex tag queries work properly

**Files Created**:
- `/server/db/nostrdb/queryNostrdb.go`
- `/tests/db/nostrdb_query_test.go`

---

### Phase 5: Handler Integration (1 week)

**Objective**: Update handlers to use abstraction layer

**Tasks**:
1. ✅ Update `/server/startup.go` - Use db.NewDatabase()
2. ✅ Update `/server/handlers/event.go` - Use abstraction
3. ✅ Update `/server/handlers/req.go` - Use abstraction
4. ✅ Add feature flag logging
5. ✅ Test both backends through handlers
6. ✅ Create `/tests/integration/database_switch_test.go`

**Success Criteria**:
- Both backends work through abstraction
- Configuration selects correct backend
- No direct mongo imports in handlers
- Feature flag logging works

**Files Modified**:
- `/server/startup.go`
- `/server/handlers/event.go`
- `/server/handlers/req.go`

**Files Created**:
- `/tests/integration/database_switch_test.go`

---

### Phase 6: Maintenance Operations (1 week)

**Objective**: Implement event purging and user enumeration

**Tasks**:
1. ✅ Create `/server/db/nostrdb/purgeEvents.go`
2. ✅ Create `/server/db/nostrdb/fetchAllUsers.go`
3. ✅ Implement health monitoring
4. ✅ Test purging operations
5. ✅ Test user enumeration

**Success Criteria**:
- Event purging works on schedule
- User sync can enumerate pubkeys
- Health checks detect issues

**Files Created**:
- `/server/db/nostrdb/purgeEvents.go`
- `/server/db/nostrdb/fetchAllUsers.go`

---

### Phase 7: Testing & Documentation (2 weeks)

**Objective**: Comprehensive testing and performance benchmarking

**Tasks**:
1. ✅ Write comprehensive integration tests
2. ✅ Create benchmark suite in `/tests/benchmarks/`
3. ✅ Profile memory usage
4. ✅ Test concurrent access scenarios
5. ✅ Load testing with realistic traffic
6. ✅ Document performance characteristics
7. ✅ Complete this documentation file

**Success Criteria**:
- 100% feature parity with MongoDB
- No memory leaks under load
- Query performance improvement documented
- All tests pass for both backends

**Files Created**:
- `/tests/benchmarks/mongodb_bench_test.go`
- `/tests/benchmarks/nostrdb_bench_test.go`
- `/tests/integration/load_test.go`

---

## Critical Technical Challenges

### Challenge 1: MongoDB Aggregation → nostrdb Queries

**Problem**: MongoDB uses `$unionWith` for cross-collection queries, nostrdb has unified storage

**MongoDB Example**:
```go
pipeline := []bson.M{
    {"$match": query},
    {"$unionWith": bson.M{"coll": "event-kind1"}},
    {"$unionWith": bson.M{"coll": "event-kind3"}},
    {"$sort": bson.M{"created_at": -1}},
    {"$limit": 500},
}
```

**nostrdb Solution**:
```go
// Simpler - no collections needed
query := ndb.NewQuery()
query.WithKinds([]int{1, 3})  // Direct kind filter
query.SortByCreatedAt(desc)
query.Limit(500)
results := ndb.Query(query)
```

**Advantage**: nostrdb simplifies this significantly

---

### Challenge 2: Tag Filtering

**Problem**: MongoDB uses `$elemMatch`, nostrdb has different API

**MongoDB**:
```go
filter := bson.M{
    "tags": bson.M{
        "$elemMatch": bson.M{
            "0": "e",
            "1": bson.M{"$in": ["id1", "id2"]},
        },
    },
}
```

**nostrdb Solution**:
```go
// Option 1: If nostrdb supports OR on tag values
query.WithTag("e", []string{"id1", "id2"})

// Option 2: If only single value supported, merge results
results := []Event{}
for _, value := range []string{"id1", "id2"} {
    events := ndb.QueryByTag("e", value)
    results = append(results, events...)
}
results = deduplicate(results)
```

---

### Challenge 3: Replaceable Event Race Conditions

**Problem**: Concurrent replacements need atomicity

**Solution**: Use LMDB transactions
```go
func (ndb *NostrDB) replaceEvent(evt Event) error {
    return ndb.withTransaction(func(txn *C.struct_ndb_txn) error {
        // 1. Query existing (locked by transaction)
        existing, _ := queryReplaceableEvent(txn, evt.PubKey, evt.Kind)

        // 2. Compare timestamps
        if existing != nil && existing.CreatedAt > evt.CreatedAt {
            return errors.New("newer event exists")
        }

        // 3. Delete old + insert new (atomic)
        if existing != nil {
            deleteEvent(txn, existing.ID)
        }
        insertEvent(txn, evt)

        return nil
    })
}
```

---

### Challenge 4: Memory Management

**Problem**: CGo memory leaks

**Solution**: Strict defer patterns
```go
func safeOperation(evt Event) error {
    // ALWAYS use defer for C allocations
    cStr := C.CString(evt.Content)
    defer C.free(unsafe.Pointer(cStr))

    cNote, err := convertEvent(evt)
    if err != nil {
        return err // defer still executes!
    }
    defer C.ndb_note_free(cNote)

    return performOperation(cNote)
}
```

**Testing**:
```bash
# Run with race detector
go test -race ./server/db/nostrdb/

# Memory profiling
go test -memprofile=mem.out ./server/db/nostrdb/
go tool pprof -alloc_space mem.out
```

---

## File Modifications

### Files to Create (25 files)

**Database Abstraction (3)**:
1. `/server/db/interface.go`
2. `/server/db/factory.go`
3. `/server/db/mongo_adapter.go`

**nostrdb Implementation (13)**:
4. `/server/db/nostrdb/dbNostrdb.go`
5. `/server/db/nostrdb/storeNostrdb.go`
6. `/server/db/nostrdb/queryNostrdb.go`
7. `/server/db/nostrdb/checkDuplicate.go`
8. `/server/db/nostrdb/purgeEvents.go`
9. `/server/db/nostrdb/fetchAllUsers.go`
10. `/server/db/nostrdb/cgo_wrapper.go`
11. `/server/db/nostrdb/types.go`
12. `/server/db/nostrdb/transactions.go`
13. `/server/db/nostrdb/eventStore/regular.go`
14. `/server/db/nostrdb/eventStore/replaceable.go`
15. `/server/db/nostrdb/eventStore/addressable.go`
16. `/server/db/nostrdb/eventStore/delete.go`
17. `/server/db/nostrdb/eventStore/deprecated.go`
18. `/server/db/nostrdb/eventStore/unknown.go`

**Tests (9)**:
19. `/tests/db/abstraction_test.go`
20. `/tests/db/mongodb_test.go`
21. `/tests/db/nostrdb_test.go`
22. `/tests/db/nostrdb_memory_test.go`
23. `/tests/db/nostrdb_query_test.go`
24. `/tests/integration/nostrdb_storage_test.go`
25. `/tests/integration/database_switch_test.go`
26. `/tests/benchmarks/mongodb_bench_test.go`
27. `/tests/benchmarks/nostrdb_bench_test.go`

### Files to Modify (5 files)

**Configuration (2)**:
1. `/config/types/serverConfig.go` - Add Database config section
2. `/docs/examples/config.example.yml` - Add nostrdb config example

**Handlers (3)**:
3. `/server/startup.go` - Database factory initialization
4. `/server/handlers/event.go` - Use db abstraction (lines 95, 114)
5. `/server/handlers/req.go` - Use db abstraction (line 114)

---

## Performance Tuning

### LMDB Configuration

**Map Size**:
```yaml
map_size: 10737418240  # 10GB default
```
- Set to expected database size
- LMDB uses memory-mapped files (doesn't allocate all at once)
- Larger is better (cheap on 64-bit systems)

**Max Readers**:
```yaml
max_readers: 126  # Default
```
- One reader per concurrent query
- Set based on expected concurrent WebSocket connections
- Maximum: 126 (LMDB limitation)

**Max Databases**:
```yaml
max_databases: 128
```
- Number of separate LMDB databases within the environment
- nostrdb might use multiple databases for different indexes

### Query Optimization

**Indexes**:
- nostrdb should have indexes on: `id`, `pubkey`, `kind`, `created_at`, tags
- Verify indexes are used: Check nostrdb source or documentation

**Query Patterns**:
```go
// GOOD: Uses indexes
filter := nostr.Filter{
    Kinds: []int{1, 3},       // Kind index
    Authors: []string{pubkey}, // Pubkey index
    Since: timestamp,          // Created_at index
}

// BAD: Full scan if nostrdb doesn't optimize
filter := nostr.Filter{
    Tags: map[string][]string{
        "custom": []string{"value"}, // May not be indexed
    },
}
```

### Memory Usage

**CGo Overhead**:
- Each Go → C call has overhead
- Batch operations when possible
- Consider connection pooling if nostrdb supports it

**Object Pooling**:
```go
var eventPool = sync.Pool{
    New: func() interface{} {
        return new(Event)
    },
}

func processEvent(data []byte) error {
    evt := eventPool.Get().(*Event)
    defer eventPool.Put(evt)

    // Use evt...
}
```

---

## Troubleshooting

### Build Issues

**Problem**: `cannot find -lnostrdb`

**Solution**:
```bash
# Build nostrdb C library first
cd vendor/nostrdb
make

# Verify libnostrdb.a exists
ls -la build/libnostrdb.a
```

---

**Problem**: `lmdb.h: No such file or directory`

**Solution**:
```bash
# Install LMDB development headers
sudo apt-get install liblmdb-dev  # Debian/Ubuntu
brew install lmdb                  # macOS
```

---

**Problem**: CGo compilation fails on macOS

**Solution**:
```bash
# Set LMDB library path
export CGO_CFLAGS="-I/opt/homebrew/include"
export CGO_LDFLAGS="-L/opt/homebrew/lib"
go build
```

---

### Runtime Issues

**Problem**: `failed to initialize nostrdb: map full`

**Solution**: Increase `map_size` in config.yml
```yaml
nostrdb:
  map_size: 21474836480  # Increase to 20GB
```

---

**Problem**: Memory leak detected

**Solution**:
1. Check all `C.CString()` have `defer C.free()`
2. Run memory profiler:
   ```bash
   go test -memprofile=mem.out ./server/db/nostrdb/
   go tool pprof mem.out
   > list functionName
   ```

---

**Problem**: `MDB_READERS_FULL: Too many readers`

**Solution**: Increase `max_readers` in config.yml
```yaml
nostrdb:
  max_readers: 256  # Increase limit
```

---

### Query Issues

**Problem**: Queries slower than MongoDB

**Checklist**:
1. ✅ Verify indexes are created
2. ✅ Check query uses indexed fields
3. ✅ Profile query execution
4. ✅ Consider caching frequently queried data

---

**Problem**: Tag queries not working

**Solution**:
1. Check nostrdb tag index API
2. Verify tag format matches nostrdb expectations
3. Test with simple tag query first

---

## Data Migration

### Export from MongoDB

```bash
# Export all events to JSON
mongosh grain --eval "
    db.getCollectionNames()
        .filter(name => name.startsWith('event-kind'))
        .forEach(collection => {
            db[collection].find().forEach(doc => {
                print(JSON.stringify(doc));
            });
        });
" > events_export.jsonl
```

### Import to nostrdb

```go
// File: /server/utils/migration/import_events.go

package migration

import (
    "bufio"
    "encoding/json"
    "os"
    "github.com/0ceanslim/grain/server/db/nostrdb"
    nostr "github.com/0ceanslim/grain/server/types"
)

func ImportEvents(ndb *nostrdb.NostrDB, jsonlPath string) error {
    file, err := os.Open(jsonlPath)
    if err != nil {
        return err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    count := 0

    for scanner.Scan() {
        var evt nostr.Event
        if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
            log.Warn("Failed to parse event: %v", err)
            continue
        }

        if err := ndb.StoreEvent(context.TODO(), evt, nil); err != nil {
            log.Warn("Failed to store event %s: %v", evt.ID, err)
            continue
        }

        count++
        if count%1000 == 0 {
            log.Info("Imported %d events", count)
        }
    }

    log.Info("Migration complete: %d events imported", count)
    return nil
}
```

### Running Migration

```bash
# 1. Export from MongoDB
./scripts/export_mongodb.sh > events_export.jsonl

# 2. Configure nostrdb
vim config.yml  # Set database.type = "nostrdb"

# 3. Run import
go run server/utils/migration/import_events.go events_export.jsonl

# 4. Verify
go run server/utils/migration/verify_migration.go
```

---

## Success Metrics

### Functional Requirements
- [ ] All event types store correctly
- [ ] All query filters work correctly
- [ ] Event purging works
- [ ] Duplicate detection works
- [ ] Both backends pass identical test suite

### Performance Requirements
- [ ] Query latency ≤ MongoDB (target: 50% reduction)
- [ ] Write throughput ≥ MongoDB
- [ ] Memory usage ≤ MongoDB (target: 30% reduction)
- [ ] Zero memory leaks under continuous operation

### Quality Requirements
- [ ] Code coverage > 80%
- [ ] All CGo calls have proper error handling
- [ ] Documentation complete
- [ ] Migration guide tested on real deployment

---

## Timeline Summary

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| 1 | 2 weeks | Abstraction layer, MongoDB adapter |
| 2 | 2 weeks | CGo wrapper, type conversions |
| 3 | 3 weeks | Event storage operations |
| 4 | 3 weeks | Query implementation |
| 5 | 1 week | Handler integration |
| 6 | 1 week | Maintenance operations |
| 7 | 2 weeks | Testing & optimization |
| **Total** | **14 weeks** | **Production-ready nostrdb integration** |

---

## Next Steps

1. **Set up git submodule**:
   ```bash
   git submodule add https://github.com/damus-io/nostrdb vendor/nostrdb
   ```

2. **Install LMDB**:
   ```bash
   sudo apt-get install liblmdb-dev
   ```

3. **Create database abstraction interface**:
   `/server/db/interface.go`

4. **Start CGo wrapper development**:
   `/server/db/nostrdb/cgo_wrapper.go`

5. **Proceed through implementation phases systematically**

---

**Document Version**: 1.0
**Last Updated**: 2025-12-13
**Maintainer**: GRAIN Development Team
