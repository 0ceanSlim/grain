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

// DefaultFulltextKinds is the kind set grain indexes for NIP-50 search
// when config.yml doesn't say otherwise: profile metadata, text notes
// and long-form articles.
var DefaultFulltextKinds = []int{0, 1, 30023}

// MaxFulltextKinds mirrors NDB_MAX_FULLTEXT_KINDS in nostrdb.h.
const MaxFulltextKinds = 64

// Options carries everything Open needs beyond the directory.
type Options struct {
	MapSizeMB     int   // LMDB map ceiling in MB
	IngestThreads int   // 0 = nostrdb's default (one per core)
	Flags         int   // ndb_config_set_flags bitmask (FlagSkipNoteVerify, ...)
	FulltextKinds []int // kinds tokenized for NIP-50 search; nil = DefaultFulltextKinds
}

// Open initializes a new nostrdb database at the given directory path.
// mapSizeMB sets the maximum database size in megabytes (LMDB map size).
// ingestThreads controls how many threads nostrdb uses to process incoming events.
func Open(dbDir string, mapSizeMB int, ingestThreads int) (*NDB, error) {
	return OpenWithOptions(dbDir, Options{MapSizeMB: mapSizeMB, IngestThreads: ingestThreads})
}

// OpenWithFlags is like Open but forwards an ndb_config_set_flags bitmask to
// nostrdb. Use FlagSkipNoteVerify for trusted-source bulk imports.
func OpenWithFlags(dbDir string, mapSizeMB int, ingestThreads int, flags int) (*NDB, error) {
	return OpenWithOptions(dbDir, Options{MapSizeMB: mapSizeMB, IngestThreads: ingestThreads, Flags: flags})
}

// OpenWithOptions opens (or creates) the database at dbDir.
//
// FulltextKinds only governs notes written from now on: nostrdb neither
// backfills nor prunes the text index when the set changes, so widening it
// on a populated relay only makes new events of the added kinds searchable.
func OpenWithOptions(dbDir string, o Options) (*NDB, error) {
	var cfg C.struct_ndb_config
	C.ndb_default_config(&cfg)
	C.ndb_config_set_mapsize(&cfg, C.size_t(o.MapSizeMB*1024*1024))

	if o.IngestThreads > 0 {
		C.ndb_config_set_ingest_threads(&cfg, C.int(o.IngestThreads))
	}
	if o.Flags != 0 {
		C.ndb_config_set_flags(&cfg, C.int(o.Flags))
	}

	kinds := o.FulltextKinds
	if kinds == nil {
		kinds = DefaultFulltextKinds
	}
	if len(kinds) > MaxFulltextKinds {
		return nil, fmt.Errorf("fulltext_kinds lists %d kinds, nostrdb supports at most %d", len(kinds), MaxFulltextKinds)
	}
	if len(kinds) > 0 {
		ckinds := make([]C.uint64_t, len(kinds))
		for i, k := range kinds {
			if k < 0 {
				return nil, fmt.Errorf("fulltext_kinds: kind %d is negative", k)
			}
			ckinds[i] = C.uint64_t(k)
		}
		if C.ndb_config_set_fulltext_kinds(&cfg, &ckinds[0], C.int(len(ckinds))) == 0 {
			return nil, fmt.Errorf("ndb_config_set_fulltext_kinds rejected %d kinds", len(ckinds))
		}
	} else {
		// an explicit empty list disables the text index entirely
		C.ndb_config_set_fulltext_kinds(&cfg, nil, 0)
	}

	cDir := C.CString(dbDir)
	defer C.free(unsafe.Pointer(cDir))

	var ndb *C.struct_ndb
	rc := C.ndb_init(&ndb, cDir, &cfg)
	if rc == 0 {
		return nil, fmt.Errorf("ndb_init failed for directory %s", dbDir)
	}

	log.DB().Info("nostrdb opened",
		"path", dbDir,
		"map_size_mb", o.MapSizeMB,
		"ingest_threads", o.IngestThreads,
		"fulltext_kinds", kinds)

	return &NDB{ndb: ndb, expiration: newExpirationTracker()}, nil
}

// Close shuts down the nostrdb instance and frees resources.
func (db *NDB) Close() {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.ndb != nil {
		C.ndb_destroy(db.ndb)
		db.ndb = nil
		log.DB().Info("nostrdb closed")
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

// NoteCount returns the number of stored note (event) records — the relay's
// total event count for the dashboard vitals. Cheap: reads ndb_stat's per-DB
// counters, no scan.
func (db *NDB) NoteCount() (uint64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.ndb == nil {
		return 0, fmt.Errorf("nostrdb is closed")
	}

	var stat C.struct_ndb_stat
	if C.ndb_stat(db.ndb, &stat) == 0 {
		return 0, fmt.Errorf("ndb_stat failed")
	}
	return uint64(stat.dbs[C.NDB_DB_NOTE].count), nil
}

// KindStat is one bucket of the stored-event kind distribution: how many events
// of that kind and how much storage (keys + values) they occupy.
type KindStat struct {
	Kind  int    `json:"kind"` // representative kind number; -1 for category/other
	Name  string `json:"name"`
	Count uint64 `json:"count"`
	Bytes uint64 `json:"bytes"` // key_size + value_size for this bucket
}

// ckindToKind maps a common-kind bucket index to a representative kind number.
// Order MUST match enum ndb_common_kind in nostrdb.h. Buckets that are a
// category rather than a single kind (LIST) use -1.
var ckindToKind = []int{0, 1, 3, 4, 5, 6, 7, 9735, 9734, 23194, 23195, 27235, -1, 30023, 30315}

// KindDistribution returns this relay's stored-event counts grouped by nostrdb's
// common-kind buckets (Profile, Text, Contacts, Repost, Reaction, Zap, Longform,
// …) plus an "other" bucket for everything else. Zero-count buckets are omitted.
// Cheap: a single ndb_stat, no scan.
func (db *NDB) KindDistribution() ([]KindStat, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.ndb == nil {
		return nil, fmt.Errorf("nostrdb is closed")
	}

	var stat C.struct_ndb_stat
	if C.ndb_stat(db.ndb, &stat) == 0 {
		return nil, fmt.Errorf("ndb_stat failed")
	}

	n := int(C.NDB_CKIND_COUNT)
	out := make([]KindStat, 0, n+1)
	for i := 0; i < n; i++ {
		c := uint64(stat.common_kinds[i].count)
		if c == 0 {
			continue
		}
		name := C.GoString(C.ndb_kind_name(C.enum_ndb_common_kind(i)))
		kind := -1
		if i < len(ckindToKind) {
			kind = ckindToKind[i]
		}
		bytes := uint64(stat.common_kinds[i].key_size) + uint64(stat.common_kinds[i].value_size)
		out = append(out, KindStat{Kind: kind, Name: name, Count: c, Bytes: bytes})
	}
	if oc := uint64(stat.other_kinds.count); oc > 0 {
		ob := uint64(stat.other_kinds.key_size) + uint64(stat.other_kinds.value_size)
		out = append(out, KindStat{Kind: -1, Name: "other", Count: oc, Bytes: ob})
	}
	return out, nil
}
