package nostrdb

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo windows CFLAGS: -DSECP256K1_STATIC
#cgo LDFLAGS: -L${SRCDIR}/lib -lnostrdb_full -lpthread -lm
#cgo windows LDFLAGS: -lws2_32 -lbcrypt

#include "nostrdb.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/0ceanslim/grain/server/utils/log"
)

// NDB wraps a nostrdb instance. It is safe for concurrent use.
type NDB struct {
	ndb        *C.struct_ndb
	mu         sync.RWMutex // protects close
	expiration *ExpirationTracker
}

// NDB open flags. These map 1:1 onto nostrdb.h NDB_FLAG_* bits.
const (
	// FlagSkipNoteVerify makes the ingester skip signature verification.
	// Safe for imports from a trusted source (e.g. a previous grain/mongo
	// export) where events were already validated at original ingest time.
	FlagSkipNoteVerify = 1 << 1
)

// Open initializes a new nostrdb database at the given directory path.
// mapSizeMB sets the maximum database size in megabytes (LMDB map size).
// ingestThreads controls how many threads nostrdb uses to process incoming events.
func Open(dbDir string, mapSizeMB int, ingestThreads int) (*NDB, error) {
	return OpenWithFlags(dbDir, mapSizeMB, ingestThreads, 0)
}

// OpenWithFlags is like Open but forwards an ndb_config_set_flags bitmask to
// nostrdb. Use FlagSkipNoteVerify for trusted-source bulk imports.
func OpenWithFlags(dbDir string, mapSizeMB int, ingestThreads int, flags int) (*NDB, error) {
	var cfg C.struct_ndb_config
	C.ndb_default_config(&cfg)
	C.ndb_config_set_mapsize(&cfg, C.size_t(mapSizeMB*1024*1024))

	if ingestThreads > 0 {
		C.ndb_config_set_ingest_threads(&cfg, C.int(ingestThreads))
	}
	if flags != 0 {
		C.ndb_config_set_flags(&cfg, C.int(flags))
	}

	cDir := C.CString(dbDir)
	defer C.free(unsafe.Pointer(cDir))

	var ndb *C.struct_ndb
	rc := C.ndb_init(&ndb, cDir, &cfg)
	if rc == 0 {
		return nil, fmt.Errorf("ndb_init failed for directory %s", dbDir)
	}

	log.GetLogger("db").Info("nostrdb opened",
		"path", dbDir,
		"map_size_mb", mapSizeMB,
		"ingest_threads", ingestThreads)

	return &NDB{ndb: ndb, expiration: newExpirationTracker()}, nil
}

// Close shuts down the nostrdb instance and frees resources.
func (db *NDB) Close() {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.ndb != nil {
		C.ndb_destroy(db.ndb)
		db.ndb = nil
		log.GetLogger("db").Info("nostrdb closed")
	}
}

// ProcessEvent ingests a raw JSON Nostr event string into the database.
// nostrdb parses the JSON, validates, indexes, and stores the event internally.
// The JSON should be a relay message like: ["EVENT", <subscription_id>, <event>]
// or just the event object itself for direct ingestion.
func (db *NDB) ProcessEvent(json string) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.ndb == nil {
		return fmt.Errorf("nostrdb is closed")
	}

	cJSON := C.CString(json)
	defer C.free(unsafe.Pointer(cJSON))

	rc := C.ndb_process_event(db.ndb, cJSON, C.int(len(json)))
	if rc == 0 {
		return fmt.Errorf("ndb_process_event failed")
	}

	return nil
}

// DeleteNoteByID enqueues a real delete of an event from nostrdb by its raw
// 32-byte ID. The delete is applied by the nostrdb writer thread in FIFO order
// with ingests — a delete of an in-flight ingest of the same ID is committed
// atomically in the same batch and cannot race.
//
// This is grain's one and only physical-delete primitive. All three deletion
// audiences (NIP-09 author deletes, operator retention / PurgeOldEvents,
// replaceable/addressable supersede, and admin --delete CLI) call through
// here. Authorization is the caller's responsibility: this function performs
// no checks beyond "is the DB open and is the writer queue accepting work".
//
// Returns an error only if the DB is closed or the writer inbox is full.
// "Not found" is not an error at this layer — it's logged at C level and the
// call is a no-op.
func (db *NDB) DeleteNoteByID(id [32]byte) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.ndb == nil {
		return fmt.Errorf("nostrdb is closed")
	}

	rc := C.ndb_request_delete_note(db.ndb, (*C.uchar)(unsafe.Pointer(&id[0])))
	if rc == 0 {
		return fmt.Errorf("ndb_request_delete_note: writer queue full")
	}
	return nil
}

// ProcessEvents ingests multiple newline-delimited JSON events.
func (db *NDB) ProcessEvents(ldjson string) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.ndb == nil {
		return fmt.Errorf("nostrdb is closed")
	}

	cJSON := C.CString(ldjson)
	defer C.free(unsafe.Pointer(cJSON))

	rc := C.ndb_process_events(db.ndb, cJSON, C.size_t(len(ldjson)))
	if rc == 0 {
		return fmt.Errorf("ndb_process_events failed")
	}

	return nil
}

// Txn represents a read transaction against nostrdb.
// Transactions must be short-lived to avoid blocking LMDB space reclamation.
type Txn struct {
	txn C.struct_ndb_txn
	db  *NDB
}

// BeginQuery starts a read transaction for querying the database.
// The caller MUST call EndQuery when done.
func (db *NDB) BeginQuery() (*Txn, error) {
	db.mu.RLock()

	if db.ndb == nil {
		db.mu.RUnlock()
		return nil, fmt.Errorf("nostrdb is closed")
	}

	txn := &Txn{db: db}
	rc := C.ndb_begin_query(db.ndb, &txn.txn)
	if rc == 0 {
		db.mu.RUnlock()
		return nil, fmt.Errorf("ndb_begin_query failed")
	}

	return txn, nil
}

// EndQuery ends a read transaction. Must be called after BeginQuery.
func (txn *Txn) EndQuery() {
	C.ndb_end_query(&txn.txn)
	txn.db.mu.RUnlock()
}

// Stat returns database statistics.
func (db *NDB) Stat() (*C.struct_ndb_stat, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.ndb == nil {
		return nil, fmt.Errorf("nostrdb is closed")
	}

	var stat C.struct_ndb_stat
	rc := C.ndb_stat(db.ndb, &stat)
	if rc == 0 {
		return nil, fmt.Errorf("ndb_stat failed")
	}

	return &stat, nil
}
