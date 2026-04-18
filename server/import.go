package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	nostr "github.com/0ceanslim/grain/server/types"
)

// importBatchLimit caps how many events we ingest per process invocation.
// The vendored nostrdb ingester crashes around ~29-30k ingested events per
// process lifetime. Setting this below that threshold lets each run exit
// cleanly. Re-run the same command to continue — nostrdb deduplicates
// already-imported events automatically.
const importBatchLimit = 25000

// ImportEvents reads a JSONL file of Nostr events and stores them in nostrdb.
// It processes up to importBatchLimit events per invocation, then exits
// cleanly. Re-run the same command to continue importing.
func ImportEvents(filename string) error {
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

	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open import file: %w", err)
	}
	defer file.Close()

	fmt.Printf("Importing events from %s into %s...\n\n", filename, dbPath)

	db, err := nostrdb.OpenWithFlags(dbPath, mapSizeMB, 1, nostrdb.FlagSkipNoteVerify)
	if err != nil {
		return fmt.Errorf("failed to open nostrdb: %w", err)
	}
	nostrdb.SetGlobalDB(db)
	defer func() {
		// Let the writer thread finish committing before exit.
		time.Sleep(3 * time.Second)
		db.Close()
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4*1024), 10*1024*1024) // up to 10MB per line

	ctx := context.Background()
	var totalLines, imported, skipped, errors int
	startTime := time.Now()
	batchLimitHit := false

	for scanner.Scan() {
		line := scanner.Bytes()
		totalLines++

		if len(line) == 0 {
			continue
		}

		var evt nostr.Event
		if err := json.Unmarshal(line, &evt); err != nil {
			fmt.Fprintf(os.Stderr, "  Line %d: failed to parse JSON: %v (skipping)\n", totalLines, err)
			skipped++
			continue
		}

		if evt.ID == "" || evt.PubKey == "" || evt.Sig == "" {
			fmt.Fprintf(os.Stderr, "  Line %d: missing required fields (skipping)\n", totalLines)
			skipped++
			continue
		}

		if err := db.StoreEvent(ctx, evt); err != nil {
			errors++
			continue
		}

		imported++

		if imported%1000 == 0 {
			fmt.Printf("  Progress: %d events imported (%d lines processed)\n", imported, totalLines)
		}

		if imported >= importBatchLimit {
			batchLimitHit = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nImport complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Total lines:  %d\n", totalLines)
	fmt.Printf("  Imported:     %d\n", imported)
	fmt.Printf("  Skipped:      %d (parse errors / missing fields)\n", skipped)
	fmt.Printf("  Store errors: %d (duplicates / rejected replacements)\n", errors)

	if batchLimitHit {
		fmt.Printf("\n  Batch limit reached (%d). Run the command again to continue.\n", importBatchLimit)
	}

	return nil
}
