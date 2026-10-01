package arrivals

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func id(n int) string {
	return strings.Repeat("0", 62) + string("0123456789abcdef"[n/16]) + string("0123456789abcdef"[n%16])
}

func TestLedger_FirstArrivalWins(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "arrivals.log"))
	if err != nil {
		t.Fatal(err)
	}
	l.Record(id(1), 100)
	l.Record(id(1), 200)
	if got, ok := l.ArrivedAt(id(1)); !ok || got != 100 {
		t.Fatalf("ArrivedAt = %d, %v; want 100, true", got, ok)
	}
	if _, ok := l.ArrivedAt(id(2)); ok {
		t.Fatal("unrecorded id reported as arrived")
	}
}

func TestLedger_FlushPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arrivals.log")
	l, _ := Open(path)
	l.Record(id(1), 100)
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	l.Record(id(2), 200)
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}

	back, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Len() != 2 {
		t.Fatalf("reopened ledger has %d entries, want 2", back.Len())
	}
	if got, _ := back.ArrivedAt(id(2)); got != 200 {
		t.Fatalf("ArrivedAt(2) = %d after reopen", got)
	}
}

func TestLedger_PruneRewrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arrivals.log")
	l, _ := Open(path)
	l.Record(id(1), 100)
	l.Record(id(2), 200)
	l.Flush()
	l.Record(id(3), 300) // pending, not yet flushed

	n, err := l.Prune(150)
	if err != nil || n != 1 {
		t.Fatalf("Prune = %d, %v; want 1, nil", n, err)
	}
	if _, ok := l.ArrivedAt(id(1)); ok {
		t.Fatal("pruned entry still present")
	}

	// The rewrite covered the pending entry; a Flush must not duplicate it.
	l.Flush()
	raw, _ := os.ReadFile(path)
	if got := strings.Count(string(raw), "\n"); got != 2 {
		t.Fatalf("file has %d lines after prune, want 2:\n%s", got, raw)
	}
	back, _ := Open(path)
	if back.Len() != 2 {
		t.Fatalf("reopened ledger has %d entries, want 2", back.Len())
	}
}

func TestLedger_SkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arrivals.log")
	body := id(1) + " 100\n" + "short 1\n" + strings.Repeat("z", 64) + " 1\n" + id(2) + " x\n" + id(3) + " 300"
	os.WriteFile(path, []byte(body), 0o644)
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 {
		t.Fatalf("loaded %d entries, want 2 (ids 1 and 3)", l.Len())
	}
}

func TestLedger_Concurrent(t *testing.T) {
	l, _ := Open(filepath.Join(t.TempDir(), "arrivals.log"))
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 64; i++ {
				l.Record(id((w*64+i)%256), int64(i))
				if i%16 == 0 {
					l.Flush()
					l.Prune(4)
				}
			}
		}(w)
	}
	wg.Wait()
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
}
