package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"encoding/hex"
	"encoding/json"
	"fmt"

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
