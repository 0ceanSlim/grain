// Package arrivals records when the relay received events that were already
// old on arrival, so retention can age them from receipt instead of from
// their created_at (event_purge.retention_clock: received).
//
// Only late arrivals are recorded. An event received within the late-arrival
// threshold of its created_at is aged from created_at, which is at most that
// much early, so the ledger holds the late arrivals of one keep window rather
// than every event. It persists as an append-only file of
// "<event id> <unix seconds>" lines: Flush appends what was recorded since
// the last flush, and Prune rewrites the file without expired entries.
package arrivals

import (
	"bufio"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Ledger maps event ids to the unix time they arrived. Safe for concurrent
// use.
type Ledger struct {
	path string

	mu      sync.Mutex
	at      map[string]int64
	pending []string // lines recorded since the last write

	fileMu sync.Mutex // serializes Flush's append against Prune's rewrite
}

// Open loads the ledger at path. A missing file is an empty ledger;
// malformed lines are skipped.
func Open(path string) (*Ledger, error) {
	l := &Ledger{path: path, at: make(map[string]int64)}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if id, t, ok := parseLine(sc.Text()); ok {
			l.at[id] = t
		}
	}
	return l, sc.Err()
}

// Record notes that id arrived at t. The first arrival wins.
func (l *Ledger) Record(id string, t int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.at[id]; ok {
		return
	}
	l.at[id] = t
	l.pending = append(l.pending, formatLine(id, t))
}

// ArrivedAt returns when id arrived, if its arrival was recorded.
func (l *Ledger) ArrivedAt(id string) (int64, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.at[id]
	return t, ok
}

// Len returns the number of recorded arrivals.
func (l *Ledger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.at)
}

// Flush appends the arrivals recorded since the last write. On failure they
// stay queued for the next attempt.
func (l *Ledger) Flush() error {
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	l.mu.Lock()
	lines := l.pending
	l.pending = nil
	l.mu.Unlock()
	if len(lines) == 0 {
		return nil
	}

	err := appendLines(l.path, lines)
	if err != nil {
		l.mu.Lock()
		l.pending = append(lines, l.pending...)
		l.mu.Unlock()
	}
	return err
}

// Prune forgets arrivals at or before cutoff, which no longer keep anything
// from being purged, and rewrites the file with what remains. It returns how
// many it forgot.
func (l *Ledger) Prune(cutoff int64) (int, error) {
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	l.mu.Lock()
	n := 0
	for id, t := range l.at {
		if t <= cutoff {
			delete(l.at, id)
			n++
		}
	}
	if n == 0 {
		l.mu.Unlock()
		return 0, nil
	}
	// The rewrite carries every live entry, so it also covers what was
	// pending; restore that on failure so a later Flush still writes it.
	taken := l.pending
	l.pending = nil
	lines := make([]string, 0, len(l.at))
	for id, t := range l.at {
		lines = append(lines, formatLine(id, t))
	}
	l.mu.Unlock()

	if err := rewrite(l.path, lines); err != nil {
		l.mu.Lock()
		l.pending = append(taken, l.pending...)
		l.mu.Unlock()
		return n, err
	}
	return n, nil
}

func formatLine(id string, t int64) string {
	return id + " " + strconv.FormatInt(t, 10) + "\n"
}

func parseLine(s string) (string, int64, bool) {
	id, ts, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok || len(id) != 64 {
		return "", 0, false
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", 0, false
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return "", 0, false
	}
	return id, t, true
}

func appendLines(path string, lines []string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(strings.Join(lines, "")); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func rewrite(path string, lines []string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "")), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
