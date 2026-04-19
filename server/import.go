package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	nostr "github.com/0ceanslim/grain/server/types"
)

// ImportEvents reads a JSONL file of Nostr events and stores them in nostrdb.
// It processes the entire file in a single run with an in-place progress bar.
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

	// First pass: count lines for accurate progress.
	fmt.Printf("Counting events in %s...", filename)
	totalLines, err := countLines(file)
	if err != nil {
		return fmt.Errorf("failed to count lines: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}
	fmt.Printf(" %d lines\n", totalLines)

	fmt.Printf("Importing into %s\n\n", dbPath)

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
	var linesRead, imported, skipped, errors int
	startTime := time.Now()
	lastRender := time.Now()
	backoff := time.Millisecond

	const maxBackoff = 100 * time.Millisecond
	const maxRetries = 500

	for scanner.Scan() {
		line := scanner.Bytes()
		linesRead++

		if len(line) == 0 {
			continue
		}

		var evt nostr.Event
		if err := json.Unmarshal(line, &evt); err != nil {
			skipped++
			continue
		}

		if evt.ID == "" || evt.PubKey == "" || evt.Sig == "" {
			skipped++
			continue
		}

		// Retry with exponential backoff on transient ingest failures
		// (queue full). Permanent rejections (duplicates, replaceable
		// conflicts) are not retried.
		stored := false
		for attempt := 0; attempt < maxRetries; attempt++ {
			err := db.StoreEvent(ctx, evt)
			if err == nil {
				stored = true
				backoff = time.Millisecond // reset on success
				break
			}
			// "blocked:" prefix = permanent rejection (duplicate,
			// replaceable conflict). Don't retry.
			if strings.HasPrefix(err.Error(), "blocked:") {
				errors++
				break
			}
			// Transient failure — back off and retry.
			if attempt < maxRetries-1 {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			errors++
		}

		if stored {
			imported++
		}

		// Update progress bar at most every 100ms to avoid terminal overhead.
		if time.Since(lastRender) >= 100*time.Millisecond {
			renderProgress(linesRead, totalLines, imported, skipped, errors, startTime)
			lastRender = time.Now()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Final progress render.
	renderProgress(linesRead, totalLines, imported, skipped, errors, startTime)
	fmt.Println() // newline after the \r progress bar

	elapsed := time.Since(startTime)
	rate := float64(0)
	if elapsed.Seconds() > 0 {
		rate = float64(imported) / elapsed.Seconds()
	}

	fmt.Printf("\nImport complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Total lines:  %d\n", linesRead)
	fmt.Printf("  Imported:     %d\n", imported)
	fmt.Printf("  Skipped:      %d (parse errors / missing fields)\n", skipped)
	fmt.Printf("  Store errors: %d (duplicates / rejected replacements)\n", errors)
	fmt.Printf("  Throughput:   %.0f events/sec\n", rate)

	return nil
}

// countLines counts the number of newline-delimited lines in r.
func countLines(r io.Reader) (int, error) {
	buf := make([]byte, 64*1024)
	count := 0
	for {
		n, err := r.Read(buf)
		count += bytes.Count(buf[:n], []byte{'\n'})
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
	}
}

// renderProgress prints an in-place progress bar using \r.
func renderProgress(current, total, imported, skipped, errors int, start time.Time) {
	elapsed := time.Since(start).Seconds()
	rate := float64(0)
	if elapsed > 0 {
		rate = float64(current) / elapsed
	}

	pct := float64(0)
	eta := ""
	if total > 0 {
		pct = float64(current) / float64(total) * 100
		remaining := float64(total-current) / rate
		if rate > 0 && remaining < 86400 {
			if remaining < 60 {
				eta = fmt.Sprintf("ETA %ds", int(remaining))
			} else if remaining < 3600 {
				eta = fmt.Sprintf("ETA %dm%ds", int(remaining)/60, int(remaining)%60)
			} else {
				eta = fmt.Sprintf("ETA %dh%dm", int(remaining)/3600, (int(remaining)%3600)/60)
			}
		}
	}

	// Progress bar: 25 chars wide.
	barWidth := 25
	filled := 0
	if total > 0 {
		filled = int(float64(barWidth) * float64(current) / float64(total))
		if filled > barWidth {
			filled = barWidth
		}
	}

	bar := make([]byte, barWidth)
	for i := range bar {
		if i < filled {
			bar[i] = '='
		} else if i == filled {
			bar[i] = '>'
		} else {
			bar[i] = ' '
		}
	}

	fmt.Fprintf(os.Stderr, "\r[%s] %5.1f%%  %d / %d  |  ok: %d  skip: %d  err: %d  |  %.0f/s  %s   ",
		string(bar), pct, current, total, imported, skipped, errors, rate, eta)
}
