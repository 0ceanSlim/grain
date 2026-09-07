package nostrdb

/*
#include "nostrdb.h"
*/
import "C"
import (
	"context"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// Map-usage thresholds. WARN/ERROR drive the periodic gauge; RejectFraction is
// where the write path starts refusing new events so the map never actually
// fills (a full LMDB map makes the writer thread fail silently while events are
// still acked). The gap below 1.0 leaves room for the purge's own delete
// transactions, which need free pages too.
const (
	MapUsageWarnFraction   = 0.80
	MapUsageErrorFraction  = 0.95
	MapUsageRejectFraction = 0.97
)

// MapUsage returns the LMDB map's used and total (ceiling) bytes. ok is false
// when the stats can't be read (e.g. the DB is closed).
func (db *NDB) MapUsage() (used, total uint64, ok bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.ndb == nil {
		return 0, 0, false
	}
	var cUsed, cTotal C.size_t
	if C.ndb_map_usage(db.ndb, &cUsed, &cTotal) == 0 {
		return 0, 0, false
	}
	return uint64(cUsed), uint64(cTotal), true
}

// MapUsageFraction returns used/total in [0,1], or 0 if unavailable. Cheap
// enough (two txn-less LMDB header reads) to call on the write path.
func (db *NDB) MapUsageFraction() float64 {
	used, total, ok := db.MapUsage()
	if !ok || total == 0 {
		return 0
	}
	return float64(used) / float64(total)
}

// WriteErrorCount returns nostrdb's running total of writer failures (map full,
// bad txn, etc.). The writer thread only logs these to stderr and events are
// acked OK before the write commits, so polling this is how grain notices
// silent write loss. It's a process-global count, valid regardless of db state.
func (db *NDB) WriteErrorCount() uint64 {
	return uint64(C.ndb_write_error_count())
}

// StartMapUsageMonitor logs the LMDB map usage at startup and every interval,
// escalating WARN at 80% and ERROR at 95% so a filling database is visible in
// grain's own logs (the writer's MDB_MAP_FULL errors only reach stderr).
// Bounded to ctx like the other background loops.
func (db *NDB) StartMapUsageMonitor(ctx context.Context, interval time.Duration) {
	go func() {
		log.DB().Info("Map usage monitor started", "interval", interval)
		db.logMapUsage()
		lastWriteErrs := db.WriteErrorCount()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.DB().Info("Map usage monitor stopping")
				return
			case <-ticker.C:
				db.logMapUsage()
				// Surface silent write loss: the writer only logs to stderr and
				// events are acked before they commit, so a climbing count means
				// events were accepted but not stored.
				if cur := db.WriteErrorCount(); cur > lastWriteErrs {
					log.DB().Error("nostrdb writer failures detected — accepted events may not have been stored",
						"new_failures", cur-lastWriteErrs, "total_failures", cur)
					lastWriteErrs = cur
				}
			}
		}
	}()
}

func (db *NDB) logMapUsage() {
	used, total, ok := db.MapUsage()
	if !ok || total == 0 {
		return
	}
	frac := float64(used) / float64(total)
	usedMiB := used >> 20
	mapMiB := total >> 20
	pct := int(frac * 100)
	switch {
	case frac >= MapUsageErrorFraction:
		log.DB().Error("Database map nearly full — writes will be rejected soon",
			"used_mib", usedMiB, "map_mib", mapMiB, "used_pct", pct)
	case frac >= MapUsageWarnFraction:
		log.DB().Warn("Database map filling",
			"used_mib", usedMiB, "map_mib", mapMiB, "used_pct", pct)
	default:
		log.DB().Info("Database map usage",
			"used_mib", usedMiB, "map_mib", mapMiB, "used_pct", pct)
	}
}
