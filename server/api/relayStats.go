package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetRelayStats serves a compact, public snapshot of the relay's live vitals for
// the dashboard: storage fill, total events, active connections, uptime, and
// build version. Storage, connections and write errors are cheap live reads;
// the event total and per-kind breakdown come from a cached ndb_stat snapshot
// (see eventStatsCache), so no request ever pays for a full-database walk.
// Client-pool / NIP-66 discovery counts come from the separate
// /api/v1/client/status; this is the relay-side half.
//
// @Summary      Relay vitals
// @Description  Storage fill, event count, connections, uptime, version — for the dashboard. Event counts refresh at most every 30s.
// @Tags         relay
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /api/v1/relay/stats [get]
func GetRelayStats(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"server": statsHook(), // connections, uptime, version (wired in startup)
	}

	if db := nostrdb.GetDB(); db != nil {
		if used, total, ok := db.MapUsage(); ok {
			var pct float64
			if total > 0 {
				pct = float64(used) / float64(total) * 100
			}
			out["storage"] = map[string]any{
				"used_bytes":  used,
				"total_bytes": total,
				"pct":         pct,
			}
		}
		if snap := eventStats.get(db); snap != nil {
			out["events"] = map[string]any{
				"total":   snap.total,
				"by_kind": snap.byKind,
			}
		}
		// Writer-failure counter (rc2 durability): 0 = healthy, >0 = silent
		// write loss (map full / bad txn) — a real relay-health signal.
		out["write_errors"] = db.WriteErrorCount()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.RelayAPI().Error("Failed to encode relay stats", "error", err)
	}
}

const (
	eventStatsTTL       = 30 * time.Second
	eventStatsFirstWait = 2 * time.Second
)

// eventStatsCache fronts nostrdb's EventStats, which walks every entry of every
// LMDB table (~340ms at 300k events). Uncached on a public endpoint that was
// free CPU amplification, and during startup's heavy work a slow walk could
// outlast the write timeout and hand the dashboard an empty body.
//
// Stale-while-revalidate: requests always read the last snapshot; a stale one
// kicks off at most one background refresh. Only the very first request waits
// (briefly) — if even that times out, "events" is simply omitted, which the
// dashboard already tolerates.
type eventStatsCache struct {
	mu      sync.Mutex
	db      *nostrdb.NDB // snapshot belongs to this instance; a restart reopens the DB
	snap    *eventStatsSnapshot
	at      time.Time
	pending chan struct{} // non-nil while a refresh runs; closed when it ends
}

type eventStatsSnapshot struct {
	total  uint64
	byKind []nostrdb.KindStat
}

var eventStats eventStatsCache

func (c *eventStatsCache) get(db *nostrdb.NDB) *eventStatsSnapshot {
	c.mu.Lock()
	if c.db != db {
		c.db, c.snap = db, nil
	}
	snap := c.snap
	if snap == nil || time.Since(c.at) > eventStatsTTL {
		c.refreshLocked(db)
	}
	pending := c.pending
	c.mu.Unlock()

	if snap != nil || pending == nil {
		return snap
	}
	select {
	case <-pending:
	case <-time.After(eventStatsFirstWait):
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snap
}

// refreshLocked starts a background refresh unless one is already running.
func (c *eventStatsCache) refreshLocked(db *nostrdb.NDB) {
	if c.pending != nil {
		return
	}
	done := make(chan struct{})
	c.pending = done
	go func() {
		defer close(done)
		total, byKind, err := db.EventStats()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.pending = nil
		if err != nil {
			log.RelayAPI().Warn("Failed to refresh event stats", "error", err)
			return
		}
		if c.db == db {
			c.snap = &eventStatsSnapshot{total: total, byKind: byKind}
			c.at = time.Now()
		}
	}()
}
