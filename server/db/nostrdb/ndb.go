package nostrdb

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo LDFLAGS: -L${SRCDIR}/lib -lnostrdb_full -lpthread -lm
#cgo windows LDFLAGS: -lws2_32

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
	ndb *C.struct_ndb
	mu  sync.RWMutex // protects close
}

// Open initializes a new nostrdb database at the given directory path.
// mapSizeMB sets the maximum database size in megabytes (LMDB map size).
// ingestThreads controls how many threads nostrdb uses to process incoming events.
func Open(dbDir string, mapSizeMB int, ingestThreads int) (*NDB, error) {
	var cfg C.struct_ndb_config
	C.ndb_default_config(&cfg)
	C.ndb_config_set_mapsize(&cfg, C.size_t(mapSizeMB*1024*1024))

	if ingestThreads > 0 {
		C.ndb_config_set_ingest_threads(&cfg, C.int(ingestThreads))
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

	return &NDB{ndb: ndb}, nil
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
