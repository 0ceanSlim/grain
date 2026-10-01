package nostrdb

import (
	"context"
	"time"

	"github.com/0ceanslim/grain/server/db/arrivals"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// arrivalFlushInterval bounds what a crash can lose: late arrivals recorded
// since the last flush fall back to the created_at clock.
const arrivalFlushInterval = 30 * time.Second

// TrackArrivals opens the late-arrival ledger at path and starts recording:
// from now on an event stored more than late after its created_at has its
// arrival time kept, so the received retention clock can age it from then.
// The ledger is flushed every arrivalFlushInterval until ctx ends, and on
// Close.
func (db *NDB) TrackArrivals(ctx context.Context, path string, late time.Duration) error {
	l, err := arrivals.Open(path)
	if err != nil {
		return err
	}
	db.lateArrivalSecs.Store(int64(late / time.Second))
	db.arrivals.Store(l)
	log.DBPurge().Info("Tracking late arrivals",
		"path", path, "late_after", late, "loaded", l.Len())

	go func() {
		ticker := time.NewTicker(arrivalFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				db.flushArrivals()
				return
			case <-ticker.C:
				db.flushArrivals()
			}
		}
	}()
	return nil
}

// noteArrival records evt's arrival if it came in late. Called once nostrdb
// has queued the event.
func (db *NDB) noteArrival(evt nostr.Event) {
	l := db.arrivals.Load()
	if l == nil {
		return
	}
	now := time.Now().Unix()
	if now-evt.CreatedAt > db.lateArrivalSecs.Load() {
		l.Record(evt.ID, now)
	}
}

func (db *NDB) flushArrivals() {
	l := db.arrivals.Load()
	if l == nil {
		return
	}
	if err := l.Flush(); err != nil {
		log.DBPurge().Warn("Failed to save late-arrival ledger", "error", err)
	}
}
