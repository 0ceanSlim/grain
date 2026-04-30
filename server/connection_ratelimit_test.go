package server

import (
	"net/http/httptest"
	"sync"
	"testing"
)

// resetRateLimitState clears the per-IP buckets and the rejection
// aggregator between subtests so they don't bleed counts into each
// other. Tests should call this in their own setup; package-level state
// is fine for production but a hassle for table-driven tests.
func resetRateLimitState() {
	ipBuckets.Range(func(k, _ interface{}) bool { ipBuckets.Delete(k); return true })
	rejAgg.mu.Lock()
	rejAgg.maxConn = 0
	rejAgg.rateLimit = 0
	rejAgg.blocked = 0
	rejAgg.topIPs = make(map[string]int)
	rejAgg.mu.Unlock()
}

func TestCheckIPConnectionRate_AllowsBelowLimit(t *testing.T) {
	resetRateLimitState()
	const limit = 5
	for i := 0; i < limit; i++ {
		if !CheckIPConnectionRate("1.1.1.1", limit) {
			t.Fatalf("attempt %d under limit %d unexpectedly rejected", i+1, limit)
		}
	}
}

func TestCheckIPConnectionRate_RejectsOverLimit(t *testing.T) {
	resetRateLimitState()
	const limit = 5
	for i := 0; i < limit; i++ {
		CheckIPConnectionRate("2.2.2.2", limit)
	}
	if CheckIPConnectionRate("2.2.2.2", limit) {
		t.Fatal("attempt over limit was allowed")
	}
}

func TestCheckIPConnectionRate_PerIPIndependent(t *testing.T) {
	resetRateLimitState()
	const limit = 2
	// Burn IP A's bucket.
	CheckIPConnectionRate("3.3.3.3", limit)
	CheckIPConnectionRate("3.3.3.3", limit)
	if CheckIPConnectionRate("3.3.3.3", limit) {
		t.Fatal("A still allowed past limit")
	}
	// IP B should be unaffected.
	if !CheckIPConnectionRate("4.4.4.4", limit) {
		t.Fatal("B rejected despite empty bucket")
	}
}

func TestCheckIPConnectionRate_DisabledWhenZero(t *testing.T) {
	resetRateLimitState()
	for i := 0; i < 1000; i++ {
		if !CheckIPConnectionRate("5.5.5.5", 0) {
			t.Fatalf("limit=0 should disable, attempt %d rejected", i+1)
		}
	}
}

func TestCheckIPConnectionRate_RejectionFeedsAggregator(t *testing.T) {
	resetRateLimitState()
	const limit = 1
	CheckIPConnectionRate("6.6.6.6", limit) // allowed, count=1
	CheckIPConnectionRate("6.6.6.6", limit) // rejected, fed to aggregator

	rejAgg.mu.Lock()
	defer rejAgg.mu.Unlock()
	if rejAgg.rateLimit != 1 {
		t.Fatalf("rate_limit counter = %d, want 1", rejAgg.rateLimit)
	}
	if rejAgg.topIPs["6.6.6.6"] != 1 {
		t.Fatalf("topIPs[6.6.6.6] = %d, want 1", rejAgg.topIPs["6.6.6.6"])
	}
}

func TestRecordRejection_CategoryRouting(t *testing.T) {
	resetRateLimitState()
	RecordRejection("max_conn", "7.7.7.7")
	RecordRejection("max_conn", "7.7.7.7")
	RecordRejection("rate_limit", "8.8.8.8")
	RecordRejection("blocked", "9.9.9.9")
	RecordRejection("unknown_category", "10.10.10.10") // should be a no-op for counters but ip still tracked

	rejAgg.mu.Lock()
	defer rejAgg.mu.Unlock()
	if rejAgg.maxConn != 2 {
		t.Errorf("maxConn = %d, want 2", rejAgg.maxConn)
	}
	if rejAgg.rateLimit != 1 {
		t.Errorf("rateLimit = %d, want 1", rejAgg.rateLimit)
	}
	if rejAgg.blocked != 1 {
		t.Errorf("blocked = %d, want 1", rejAgg.blocked)
	}
	if rejAgg.topIPs["7.7.7.7"] != 2 {
		t.Errorf("topIPs[7.7.7.7] = %d, want 2", rejAgg.topIPs["7.7.7.7"])
	}
}

func TestRecordRejection_EmptyIPDoesNotPolluteTop(t *testing.T) {
	resetRateLimitState()
	RecordRejection("max_conn", "")
	rejAgg.mu.Lock()
	defer rejAgg.mu.Unlock()
	if rejAgg.maxConn != 1 {
		t.Errorf("maxConn = %d, want 1", rejAgg.maxConn)
	}
	if _, ok := rejAgg.topIPs[""]; ok {
		t.Error("empty IP should not be added to topIPs")
	}
}

func TestEmitAndReset_NoOpWhenEmpty(t *testing.T) {
	resetRateLimitState()
	// Should not panic, should clear nothing meaningful.
	emitAndReset()
}

func TestEmitAndReset_ResetsCounters(t *testing.T) {
	resetRateLimitState()
	for i := 0; i < 3; i++ {
		RecordRejection("max_conn", "11.11.11.11")
	}
	emitAndReset()
	rejAgg.mu.Lock()
	defer rejAgg.mu.Unlock()
	if rejAgg.maxConn != 0 || len(rejAgg.topIPs) != 0 {
		t.Fatalf("aggregator did not reset: maxConn=%d topIPs=%v", rejAgg.maxConn, rejAgg.topIPs)
	}
}

func TestEnforceConnectionRateLimit_Allowed(t *testing.T) {
	resetRateLimitState()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "12.12.12.12:1234"
	w := httptest.NewRecorder()
	if !EnforceConnectionRateLimit(w, r, 5) {
		t.Fatal("first attempt should be allowed")
	}
	if w.Code != 200 { // unchanged
		t.Errorf("status code mutated to %d on allowed path", w.Code)
	}
}

func TestEnforceConnectionRateLimit_RejectedWith429(t *testing.T) {
	resetRateLimitState()
	const limit = 1
	// Burn the bucket.
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.RemoteAddr = "13.13.13.13:1234"
	w1 := httptest.NewRecorder()
	if !EnforceConnectionRateLimit(w1, r1, limit) {
		t.Fatal("first attempt should be allowed")
	}

	// Second attempt — over limit.
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "13.13.13.13:1234"
	w2 := httptest.NewRecorder()
	if EnforceConnectionRateLimit(w2, r2, limit) {
		t.Fatal("over-limit attempt should be rejected")
	}
	if w2.Code != 429 {
		t.Errorf("expected 429, got %d", w2.Code)
	}
	if got := w2.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want 60", got)
	}
}

func TestCheckIPConnectionRate_ConcurrentSameIP(t *testing.T) {
	// Hammer one IP from many goroutines and confirm: (a) no race,
	// (b) total accepted matches limit exactly, (c) rejections are
	// reflected in the aggregator.
	resetRateLimitState()
	const limit = 50
	const goroutines = 200

	var wg sync.WaitGroup
	var allowed, rejected int
	var mu sync.Mutex
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ok := CheckIPConnectionRate("14.14.14.14", limit)
			mu.Lock()
			if ok {
				allowed++
			} else {
				rejected++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if allowed != limit {
		t.Errorf("allowed = %d, want %d (limit)", allowed, limit)
	}
	if rejected != goroutines-limit {
		t.Errorf("rejected = %d, want %d", rejected, goroutines-limit)
	}
	rejAgg.mu.Lock()
	defer rejAgg.mu.Unlock()
	if rejAgg.rateLimit != goroutines-limit {
		t.Errorf("aggregator.rateLimit = %d, want %d", rejAgg.rateLimit, goroutines-limit)
	}
}
