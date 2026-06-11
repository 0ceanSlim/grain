package core

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// newTestPool builds a pool whose dial and reader-start are stubbed, so Acquire
// can be exercised without real sockets or read/write goroutines.
func newTestPool(maxConns int) *RelayPool {
	cfg := DefaultConfig()
	if maxConns > 0 {
		cfg.MaxConnections = maxConns
	}
	rp := NewRelayPool(cfg)
	rp.dialFn = func(string, time.Duration) (*websocket.Conn, error) { return nil, nil }
	rp.startReader = func(*RelayConnection) {}
	return rp
}

// testConn is a socket-less connection for exercising the lifecycle helpers.
// close() tolerates a nil Conn; done is initialised so signalDone is safe.
func testConn(url string, leases int, idleAt time.Time) *RelayConnection {
	return &RelayConnection{
		URL:           url,
		Status:        StatusConnected,
		Subscriptions: map[string]bool{},
		writeChan:     make(chan []byte, 1),
		done:          make(chan struct{}),
		leases:        leases,
		idleAt:        idleAt,
	}
}

func leasesOf(rp *RelayPool, url string) int {
	rp.mu.RLock()
	conn := rp.connections[url]
	rp.mu.RUnlock()
	conn.mu.RLock()
	defer conn.mu.RUnlock()
	return conn.leases
}

func TestAcquireDialsThenReusesAndLeases(t *testing.T) {
	rp := newTestPool(0)
	var dials int32
	rp.dialFn = func(string, time.Duration) (*websocket.Conn, error) {
		atomic.AddInt32(&dials, 1)
		return nil, nil
	}

	if _, err := rp.Acquire("wss://a"); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := rp.Acquire("wss://a"); err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if got := atomic.LoadInt32(&dials); got != 1 {
		t.Fatalf("want 1 dial (second reuses), got %d", got)
	}
	if got := leasesOf(rp, "wss://a"); got != 2 {
		t.Fatalf("want 2 leases, got %d", got)
	}

	rp.Release("wss://a")
	if got := leasesOf(rp, "wss://a"); got != 1 {
		t.Fatalf("want 1 lease after release, got %d", got)
	}
	rp.Release("wss://a")
	if got := leasesOf(rp, "wss://a"); got != 0 {
		t.Fatalf("want 0 leases, got %d", got)
	}
	rp.mu.RLock()
	idleAt := rp.connections["wss://a"].idleAt
	rp.mu.RUnlock()
	if idleAt.IsZero() {
		t.Fatal("idleAt should be set once the last lease is released")
	}
}

func TestReleaseUnderflowGuard(t *testing.T) {
	rp := newTestPool(0)
	rp.connections["a"] = testConn("a", 1, time.Time{})

	rp.Release("a")
	rp.Release("a") // already at zero — must not underflow
	if got := leasesOf(rp, "a"); got != 0 {
		t.Fatalf("want 0 leases, got %d", got)
	}
	rp.Release("missing") // unknown url is a safe no-op (must not panic)
}

func TestAcquireBackoffAfterDialFailure(t *testing.T) {
	rp := newTestPool(0)
	var dials int32
	rp.dialFn = func(string, time.Duration) (*websocket.Conn, error) {
		atomic.AddInt32(&dials, 1)
		return nil, errors.New("boom")
	}

	if _, err := rp.Acquire("wss://dead"); err == nil {
		t.Fatal("expected dial error")
	}
	// Immediate retry must be refused by backoff, without dialing again.
	if _, err := rp.Acquire("wss://dead"); err == nil {
		t.Fatal("expected backoff error on immediate retry")
	}
	if got := atomic.LoadInt32(&dials); got != 1 {
		t.Fatalf("backoff should suppress the retry dial: dialed %d times", got)
	}
	rp.mu.RLock()
	_, hasBackoff := rp.backoff["wss://dead"]
	fails := rp.failCount["wss://dead"]
	rp.mu.RUnlock()
	if !hasBackoff || fails != 1 {
		t.Fatalf("want backoff set and failCount 1, got backoff=%v fails=%d", hasBackoff, fails)
	}
}

func TestAcquireSingleFlight(t *testing.T) {
	rp := newTestPool(0)
	var dials int32
	gate := make(chan struct{})
	rp.dialFn = func(string, time.Duration) (*websocket.Conn, error) {
		atomic.AddInt32(&dials, 1)
		<-gate // hold the dial so concurrent callers pile up behind single-flight
		return nil, nil
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = rp.Acquire("wss://a")
		}(i)
	}

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&dials) < 1 {
		select {
		case <-deadline:
			t.Fatal("dial never started")
		default:
			runtime.Gosched()
		}
	}
	close(gate)
	wg.Wait()

	if got := atomic.LoadInt32(&dials); got != 1 {
		t.Fatalf("single-flight broken: dialed %d times, want 1", got)
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
	}
	if got := leasesOf(rp, "wss://a"); got != n {
		t.Fatalf("want %d leases (one per caller), got %d", n, got)
	}
}

func TestEvictIdle(t *testing.T) {
	rp := newTestPool(0)
	old := time.Now().Add(-time.Hour)
	recent := time.Now().Add(-time.Second)

	rp.connections["idleOld"] = testConn("idleOld", 0, old)          // evict
	rp.connections["idleRecent"] = testConn("idleRecent", 0, recent) // keep (not idle long enough)
	rp.connections["leased"] = testConn("leased", 1, time.Time{})    // keep (in use)
	rp.connections["pinnedOld"] = testConn("pinnedOld", 0, old)      // keep (pinned)
	rp.pinned["pinnedOld"] = true

	n := rp.evictIdle(time.Now(), 5*time.Minute)
	if n != 1 {
		t.Fatalf("want 1 eviction, got %d", n)
	}
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	if _, ok := rp.connections["idleOld"]; ok {
		t.Error("idleOld should have been evicted")
	}
	for _, keep := range []string{"idleRecent", "leased", "pinnedOld"} {
		if _, ok := rp.connections[keep]; !ok {
			t.Errorf("%s should have been kept", keep)
		}
	}
}

func TestLruIdleVictim(t *testing.T) {
	rp := newTestPool(0)
	rp.connections["idleOld"] = testConn("idleOld", 0, time.Now().Add(-time.Hour))
	rp.connections["idleNew"] = testConn("idleNew", 0, time.Now().Add(-time.Minute))
	rp.connections["leased"] = testConn("leased", 1, time.Time{})
	rp.connections["pinnedOldest"] = testConn("pinnedOldest", 0, time.Now().Add(-2*time.Hour))
	rp.pinned["pinnedOldest"] = true

	rp.mu.Lock()
	victim := rp.lruIdleVictimLocked()
	rp.mu.Unlock()
	if victim != "idleOld" {
		t.Fatalf("want longest-idle unpinned victim 'idleOld', got %q", victim)
	}
}

func TestAcquireSoftCapEvictsIdle(t *testing.T) {
	rp := newTestPool(2) // cap of 2
	rp.connections["idle"] = testConn("idle", 0, time.Now().Add(-time.Hour))
	rp.connections["leased"] = testConn("leased", 1, time.Time{})

	if _, err := rp.Acquire("wss://new"); err != nil {
		t.Fatalf("acquire over cap: %v", err)
	}

	rp.mu.RLock()
	defer rp.mu.RUnlock()
	if _, ok := rp.connections["idle"]; ok {
		t.Error("idle connection should have been evicted to honour the soft cap")
	}
	if _, ok := rp.connections["leased"]; !ok {
		t.Error("leased connection must not be evicted")
	}
	if _, ok := rp.connections["wss://new"]; !ok {
		t.Error("newly acquired connection should be present")
	}
}

func TestPin(t *testing.T) {
	rp := newTestPool(0)
	rp.Pin("wss://a", "wss://b")
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	if !rp.pinned["wss://a"] || !rp.pinned["wss://b"] {
		t.Fatal("Pin should mark both urls pinned")
	}
}
