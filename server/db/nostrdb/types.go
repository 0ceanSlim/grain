package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
#include <string.h>

// CGO helper: extract the string pointer from ndb_str's anonymous union
static inline const char *ndb_str_str(struct ndb_str *s) { return s->str; }

// CGO helper: extract the id pointer from ndb_str's anonymous union
static inline unsigned char *ndb_str_id(struct ndb_str *s) { return s->id; }
*/
import "C"
import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unsafe"

	nostr "github.com/0ceanslim/grain/server/types"
)

// noteToEvent converts a nostrdb C note into a Go nostr.Event.
// The note must be accessed within a valid read transaction.
func noteToEvent(note *C.struct_ndb_note) (nostr.Event, error) {
	// Use ndb_note_json to serialize the note, then unmarshal into our Event type.
	// This is the safest approach since it handles all the FlatBuffer field access
	// correctly, including all packed string types that noteToEventDirect misses.
	//
	// Try 64KB first (sufficient for most events), retry with 1MB for large ones.
	bufSize := 1024 * 64
	buf := C.malloc(C.size_t(bufSize))

	rc := C.ndb_note_json(note, (*C.char)(buf), C.int(bufSize))
	if rc == 0 {
		// Buffer too small — retry with 1MB
		C.free(buf)
		bufSize = 1024 * 1024
		buf = C.malloc(C.size_t(bufSize))
		rc = C.ndb_note_json(note, (*C.char)(buf), C.int(bufSize))
		if rc == 0 {
			C.free(buf)
			return nostr.Event{}, fmt.Errorf("ndb_note_json failed (note exceeds 1MB)")
		}
	}

	jsonBytes := C.GoBytes(buf, rc)
	C.free(buf)

	var evt nostr.Event
	if err := json.Unmarshal(jsonBytes, &evt); err != nil {
		return nostr.Event{}, fmt.Errorf("failed to unmarshal note JSON: %w", err)
	}

	// Ensure tags is never nil — NIP-01 requires an array, not null.
	if evt.Tags == nil {
		evt.Tags = [][]string{}
	}

	return evt, nil
}

// eventToJSON serializes a Go Event to JSON for feeding into ndb_process_event.
// nostrdb expects the raw event JSON (not wrapped in ["EVENT", ...]).
func eventToJSON(evt nostr.Event) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}
	return string(data), nil
}

// noteToEventDirect converts a nostrdb note to Event by reading fields directly.
// This avoids JSON round-tripping and is the primary query conversion path.
// Note: only handles NDB_PACKED_STR and NDB_PACKED_ID tag value types.
func noteToEventDirect(note *C.struct_ndb_note) nostr.Event {
	evt := nostr.Event{
		ID:        hex.EncodeToString(C.GoBytes(unsafe.Pointer(C.ndb_note_id(note)), 32)),
		PubKey:    hex.EncodeToString(C.GoBytes(unsafe.Pointer(C.ndb_note_pubkey(note)), 32)),
		CreatedAt: int64(C.ndb_note_created_at(note)),
		Kind:      int(C.ndb_note_kind(note)),
		Content:   C.GoString(C.ndb_note_content(note)),
		Sig:       hex.EncodeToString(C.GoBytes(unsafe.Pointer(C.ndb_note_sig(note)), 64)),
		Tags:      [][]string{},
	}

	// Extract tags
	var iter C.struct_ndb_iterator
	C.ndb_tags_iterate_start(note, &iter)

	for C.ndb_tags_iterate_next(&iter) != 0 {
		tagCount := int(C.ndb_tag_count(iter.tag))
		if tagCount == 0 {
			continue
		}

		tag := make([]string, tagCount)
		for i := 0; i < tagCount; i++ {
			nstr := C.ndb_iter_tag_str(&iter, C.int(i))
			if nstr.flag == C.NDB_PACKED_STR {
				tag[i] = C.GoString(C.ndb_str_str(&nstr))
			} else if nstr.flag == C.NDB_PACKED_ID {
				tag[i] = hex.EncodeToString(C.GoBytes(unsafe.Pointer(C.ndb_str_id(&nstr)), 32))
			}
		}
		evt.Tags = append(evt.Tags, tag)
	}

	return evt
}

// hexToBytes converts a hex string to a 32-byte array.
// Returns nil if the hex string is invalid.
func hexToBytes32(hexStr string) ([]byte, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}
	if len(bytes) != 32 {
		return nil, fmt.Errorf("expected 32 bytes, got %d", len(bytes))
	}
	return bytes, nil
}
