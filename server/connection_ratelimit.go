package server

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Per-IP connection-attempt rate limiting and aggregated rejection logging.
//
// The relay used to log one WARN per rejected connection. v0.5.0 production
// logs hit 169,272 max-conn WARNs over four hours (peak 720/min). The
// relay correctly bounced them, but every other useful signal drowned in
// the spam. This file replaces that with two coupled mechanisms:
//
//  1. A pre-upgrade per-IP attempt limiter that runs in the HTTP handler
//     before websocket.Server.ServeHTTP, so a connection storm pays only
//     the cost of a TCP accept + 429, not a full WS upgrade.
//  2. An aggregator that batches rejection counts (max-conn, per-IP rate
//     limit, blocked-IP) and emits one WARN per minute with the top
//     offending IPs, instead of one per rejection.
//
// See issue #61 for the production data and #62 for the IP-blocklist
// follow-on that consumes RecordIPViolation.

// ipBucket tracks a single IP's connection attempts inside the current
// 60-second window. lastSeen lets the cleanup goroutine evict idle
// buckets so memory under sustained scanning is bounded.
type ipBucket struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

var (
	ipBuckets sync.Map // map[string]*ipBucket
)

// CheckIPConnectionRate consumes one attempt from the given IP's bucket.
// Returns false if the attempt would put the IP over limitPerMinute and
// must be rejected. limitPerMinute=0 disables the check.
//
// Each rejection here is also reported to RecordIPViolation so #62's
// auto-escalation has a feed to consume.
func CheckIPConnectionRate(ip string, limitPerMinute int) bool {
	if limitPerMinute <= 0 {
		return true
	}
	if ip == "" {
		// Unknown IP — don't bucket against the empty string, just allow.
		// The max-conn cap and per-client rate limiter still apply.
		return true
	}

	now := time.Now()
	v, _ := ipBuckets.LoadOrStore(ip, &ipBucket{windowStart: now})
	b := v.(*ipBucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	if now.Sub(b.windowStart) >= time.Minute {
		b.windowStart = now
		b.count = 0
	}
	b.lastSeen = now
	b.count++
	if b.count > limitPerMinute {
		RecordIPViolation(ip)
		return false
	}
	return true
}

// startBucketCleanup runs in the background, evicting per-IP buckets idle
// for more than 5 minutes. Bounds memory under address-space scanning.
func startBucketCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-5 * time.Minute)
			ipBuckets.Range(func(k, v interface{}) bool {
				b := v.(*ipBucket)
				b.mu.Lock()
				idle := b.lastSeen.Before(cutoff)
				b.mu.Unlock()
				if idle {
					ipBuckets.Delete(k)
				}
				return true
			})
		}
	}()
}

// Rejection aggregator.
//
// Three reject categories are tracked: max-conn (post-upgrade gate),
// rate-limit (pre-upgrade per-IP), and blocklist (pre-upgrade IP block,
// once #62 lands). Each tracked rejection bumps its counter and the
// per-IP top-offender map. Once per minute the aggregator emits one
// WARN summarizing all three counts and the top 5 offending IPs,
// then resets.

type rejectionStats struct {
	mu        sync.Mutex
	maxConn   int
	rateLimit int
	blocked   int
	topIPs    map[string]int
}

var rejAgg = &rejectionStats{topIPs: make(map[string]int)}

// RecordRejection bumps the appropriate counter for a rejected connection
// attempt. category is one of: "max_conn", "rate_limit", "blocked". ip is
// the offending IP (may be empty if unknown — in that case the counter
// still advances but no IP is attributed).
func RecordRejection(category, ip string) {
	rejAgg.mu.Lock()
	defer rejAgg.mu.Unlock()
	switch category {
	case "max_conn":
		rejAgg.maxConn++
	case "rate_limit":
		rejAgg.rateLimit++
	case "blocked":
		rejAgg.blocked++
	}
	if ip != "" {
		rejAgg.topIPs[ip]++
	}
}

// RecordIPViolation is the hook #62 consumes for auto-escalation. The
// rate limiter calls it on every per-IP rejection. For now it only feeds
// the aggregator; the escalation state machine arrives with #62.
func RecordIPViolation(ip string) {
	RecordRejection("rate_limit", ip)
}

// startRejectionAggregator emits one summary WARN per minute. If no
// rejections happened in the window, nothing is logged.
func startRejectionAggregator() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			emitAndReset()
		}
	}()
}

// emitAndReset snapshots the aggregator under lock, resets it, and
// (outside the lock) emits the summary WARN if anything happened.
func emitAndReset() {
	rejAgg.mu.Lock()
	if rejAgg.maxConn == 0 && rejAgg.rateLimit == 0 && rejAgg.blocked == 0 {
		rejAgg.mu.Unlock()
		return
	}
	maxConn, rateLimit, blocked := rejAgg.maxConn, rejAgg.rateLimit, rejAgg.blocked
	ips := make([]string, 0, len(rejAgg.topIPs))
	for ip := range rejAgg.topIPs {
		ips = append(ips, ip)
	}
	sort.Slice(ips, func(i, j int) bool { return rejAgg.topIPs[ips[i]] > rejAgg.topIPs[ips[j]] })
	top := make([]string, 0, 5)
	for i := 0; i < len(ips) && i < 5; i++ {
		top = append(top, ips[i]+":"+itoa(rejAgg.topIPs[ips[i]]))
	}
	rejAgg.maxConn = 0
	rejAgg.rateLimit = 0
	rejAgg.blocked = 0
	rejAgg.topIPs = make(map[string]int)
	rejAgg.mu.Unlock()

	log.RelayClient().Warn("Connection rejections (last minute)",
		"max_conn", maxConn,
		"rate_limit", rateLimit,
		"blocked", blocked,
		"top_offending_ips", top)
}

// itoa avoids a strconv import for this single call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// startConnectionRejectionInfrastructure starts the cleanup and
// aggregator goroutines. Idempotent only insofar as InitStatsMonitoring
// itself is called once at startup; do not call this directly.
func startConnectionRejectionInfrastructure() {
	startBucketCleanup()
	startRejectionAggregator()
}

// EnforceConnectionRateLimit is the HTTP middleware-style guard that runs
// before the WebSocket upgrade. It consults the per-IP bucket, and on
// rejection writes 429 with no body (cheaper than a structured response)
// and reports to the aggregator. Returns true if the request should be
// allowed to proceed to the WS upgrade.
func EnforceConnectionRateLimit(w http.ResponseWriter, r *http.Request, limitPerMinute int) bool {
	if limitPerMinute <= 0 {
		return true
	}
	ip := utils.GetClientIP(r)
	if CheckIPConnectionRate(ip, limitPerMinute) {
		return true
	}
	// CheckIPConnectionRate already routed through RecordIPViolation,
	// which credits the rate_limit counter and the top-IP map.
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusTooManyRequests)
	return false
}
