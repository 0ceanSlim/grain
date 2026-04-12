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

// ImportEvents reads a JSONL file of Nostr events and stores them in nostrdb.
// It initializes the database from config, imports all events, and prints a summary.
func ImportEvents(filename string) error {
	// Load configuration to get database path
	if err := ensureConfigFiles(); err != nil {
		return fmt.Errorf("failed to ensure config files: %w", err)
	}

	cfg, err := config.LoadConfig(config.ConfigPath("config.yml"))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve database path (same logic as runServerInstance)
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

	// Ensure database directory exists
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open nostrdb
	fmt.Printf("Opening database at %s...\n", dbPath)
	db, err := nostrdb.Open(dbPath, mapSizeMB, 4)
	if err != nil {
		return fmt.Errorf("failed to open nostrdb: %w", err)
	}
	nostrdb.SetGlobalDB(db)
	defer db.Close()

	// Open JSONL file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open import file: %w", err)
	}
	defer file.Close()

	fmt.Printf("Importing events from %s...\n\n", filename)

	scanner := bufio.NewScanner(file)
	// Set large buffer for events with big content (e.g., long-form articles)
	scanner.Buffer(make([]byte, 0, 4*1024), 10*1024*1024) // up to 10MB per line

	ctx := context.Background()
	var totalLines, imported, skipped, errors int
	startTime := time.Now()

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

		// Basic validation
		if evt.ID == "" || evt.PubKey == "" || evt.Sig == "" {
			fmt.Fprintf(os.Stderr, "  Line %d: missing required fields (skipping)\n", totalLines)
			skipped++
			continue
		}

		if err := db.StoreEvent(ctx, evt); err != nil {
			// Many "errors" are expected (duplicates, older replaceable events)
			errors++
			continue
		}

		imported++

		if imported%1000 == 0 {
			fmt.Printf("  Progress: %d events imported (%d lines processed)\n", imported, totalLines)
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

	return nil
}
