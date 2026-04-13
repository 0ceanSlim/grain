package server

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
)

// DeleteEvents is the admin-takedown entry point invoked from
// `grain --delete <id>` / `grain --delete-file <path>`. It opens nostrdb with
// the same settings as normal startup, enqueues a physical delete for every
// supplied hex event id, and waits for the writer queue to drain on Close.
//
// Authorization model: there is none. Shell access to the grain binary and
// data directory is the authorization boundary — identical to the existing
// `--import` flow. This is the moderator / legal-takedown path, not a
// protocol-level feature, and deliberately has no signature check.
//
// Each call logs one line per id: "deleted <id>" on success, "not found"
// isn't observable from the Go side (the C delete is a no-op on missing
// ids and returns success), so every well-formed id that enqueues cleanly
// is reported as deleted. Malformed ids report an error and continue.
func DeleteEvents(ids []string) error {
	if err := ensureConfigFiles(); err != nil {
		return fmt.Errorf("failed to ensure config files: %w", err)
	}

	cfg, err := config.LoadConfig(config.ConfigPath("config.yml"))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data"
	}
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(config.GetDataDir(), dbPath)
	}
	mapSizeMB := cfg.Database.MapSizeMB
	if mapSizeMB <= 0 {
		mapSizeMB = 4096
	}

	fmt.Printf("Opening database at %s...\n", dbPath)
	db, err := nostrdb.Open(dbPath, mapSizeMB, 1)
	if err != nil {
		return fmt.Errorf("failed to open nostrdb: %w", err)
	}
	nostrdb.SetGlobalDB(db)
	// db.Close() blocks on ndb_destroy which drains the writer thread,
	// so every enqueued delete is committed before we return.
	defer db.Close()

	var ok, bad int
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		idBytes, err := hexToBytes32(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: invalid hex id: %v\n", id, err)
			bad++
			continue
		}
		var id32 [32]byte
		copy(id32[:], idBytes)
		if err := db.DeleteNoteByID(id32); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: delete enqueue failed: %v\n", id, err)
			bad++
			continue
		}
		fmt.Printf("  deleted %s\n", id)
		ok++
	}

	fmt.Printf("\nDelete complete: %d enqueued, %d failed\n", ok, bad)
	if ok == 0 && bad > 0 {
		return fmt.Errorf("no events deleted")
	}
	return nil
}

// ReadDeleteFile loads a newline-delimited file of hex event ids, stripping
// blank lines and comments (lines beginning with '#'). Used by
// `grain --delete-file <path>`.
func ReadDeleteFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()

	var ids []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}
	return ids, nil
}

// hexToBytes32 is a local copy of the helper in the nostrdb package — we
// can't import the unexported one. Keeps this file CGO-free.
func hexToBytes32(hexStr string) ([]byte, error) {
	if len(hexStr) != 64 {
		return nil, fmt.Errorf("expected 64 hex chars, got %d", len(hexStr))
	}
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hi, ok1 := fromHexNibble(hexStr[i*2])
		lo, ok2 := fromHexNibble(hexStr[i*2+1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid hex char in id")
		}
		b[i] = hi<<4 | lo
	}
	return b, nil
}

func fromHexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
