package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"fmt"
	"unsafe"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// encodeForIngest serializes evt for ndb_process_event and checks,
// synchronously, that nostrdb's async ingester will take it. The ingester
// parses the JSON and re-derives the id before its signature check (which
// grain has already done), and drops a note that fails either step without
// telling anyone — after grain has acked OK. Running the same two steps
// here turns that silent loss into an OK false and an ERROR line.
//
// Callers run this before anything destructive (supersede deletes, NIP-09
// deletes), so a replacement nostrdb would refuse never costs the author
// the version it was meant to replace.
func encodeForIngest(evt nostr.Event) (string, error) {
	js := eventToJSON(evt)
	if err := checkIngestable(js); err != nil {
		log.DBStore().Error("nostrdb would drop event; refusing it",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"size_bytes", len(js),
			"reason", err)
		return "", fmt.Errorf("nostrdb cannot store event: %w", err)
	}
	return js, nil
}

// checkIngestable runs nostrdb's own parser and id derivation over a note's
// JSON. A NUL byte ends the parse early in C, so content holding one fails
// here too, which is right: nostrdb cannot store it.
func checkIngestable(js string) error {
	n := len(js)

	// Same note buffer sizing as ndb_ingester_process_event.
	bufSize := max(n*8, 4096)
	buf := C.malloc(C.size_t(bufSize))
	if buf == nil {
		return fmt.Errorf("out of memory")
	}
	defer C.free(buf)

	cJSON := C.CString(js)
	defer C.free(unsafe.Pointer(cJSON))

	var note *C.struct_ndb_note
	if C.ndb_note_from_json(cJSON, C.int(n), &note, (*C.uchar)(buf), C.int(bufSize)) <= 0 || note == nil {
		return fmt.Errorf("event JSON does not parse")
	}

	// The commitment re-escapes strings, so give it room beyond the JSON.
	scratchSize := 2*n + 4096
	scratch := C.malloc(C.size_t(scratchSize))
	if scratch == nil {
		return fmt.Errorf("out of memory")
	}
	defer C.free(scratch)

	var id [32]byte
	if C.ndb_calculate_id(note, (*C.uchar)(scratch), C.int(scratchSize), (*C.uchar)(unsafe.Pointer(&id[0]))) == 0 {
		return fmt.Errorf("could not compute event id")
	}
	if !bytes.Equal(id[:], C.GoBytes(unsafe.Pointer(C.ndb_note_id(note)), 32)) {
		return fmt.Errorf("event id does not match its content as nostrdb reads it")
	}
	return nil
}
