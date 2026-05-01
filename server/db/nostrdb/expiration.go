package nostrdb

import (
	"container/heap"
	"context"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// NIP-40 expiration tracker.
//
// `expiration` is a multi-character tag and therefore not indexed by
// nostrdb (NIP-01 indexes single-letter tags only). Querying by it would
// require a full scan, which is fine for a one-shot bootstrap but
// unworkable for steady-state sweeping. We instead keep an in-memory
// min-heap keyed by expiration timestamp, populated by:
//
//   - a one-shot startup scan (Bootstrap) that walks the entire DB and
//     deletes anything already expired; and
//   - every successful EVENT ingest with an expiration tag (Track).
//
// A background goroutine (RunSweeper) blocks on the heap top and calls
// DeleteNoteByID as items come due. Restarts re-populate the heap by
// rescanning — see issue #49 for the rationale and trade-offs.

// expirationItem is one entry in the min-heap.
type expirationItem struct {
	expireAt int64
	id       [32]byte
}

type expirationHeap []expirationItem

func (h expirationHeap) Len() int           { return len(h) }
func (h expirationHeap) Less(i, j int) bool { return h[i].expireAt < h[j].expireAt }
func (h expirationHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *expirationHeap) Push(x interface{}) {
	*h = append(*h, x.(expirationItem))
}

func (h *expirationHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// ExpirationTracker is a thread-safe min-heap of pending event
// expirations. Created and owned by NDB; do not instantiate directly.
type ExpirationTracker struct {
	mu     sync.Mutex
	heap   expirationHeap
	notify chan struct{} // wakes the sweeper when a sooner deadline is pushed
}

func newExpirationTracker() *ExpirationTracker {
	return &ExpirationTracker{
		notify: make(chan struct{}, 1),
	}
}

// Track records a pending expiration. If the new entry beats the current
// heap top, the sweeper is woken so it doesn't oversleep.
func (t *ExpirationTracker) Track(expireAt int64, id [32]byte) {
	t.mu.Lock()
	wakeup := t.heap.Len() == 0 || expireAt < t.heap[0].expireAt
	heap.Push(&t.heap, expirationItem{expireAt: expireAt, id: id})
	t.mu.Unlock()

	if wakeup {
		select {
		case t.notify <- struct{}{}:
		default:
		}
	}
}

// Len returns the current number of tracked expirations. For tests/observability.
func (t *ExpirationTracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.heap.Len()
}

// popDue removes and returns all items with expireAt <= now.
func (t *ExpirationTracker) popDue(now int64) []expirationItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	var due []expirationItem
	for t.heap.Len() > 0 && t.heap[0].expireAt <= now {
		due = append(due, heap.Pop(&t.heap).(expirationItem))
	}
	return due
}

// nextDeadline returns the earliest expireAt or (0, false) if heap is empty.
func (t *ExpirationTracker) nextDeadline() (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.heap.Len() == 0 {
		return 0, false
	}
	return t.heap[0].expireAt, true
}

// expirationFromTags extracts a NIP-40 expiration timestamp from a tag set.
// Returns (0, false) if absent or malformed.
func expirationFromTags(tags [][]string) (int64, bool) {
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == "expiration" {
			ts, err := strconv.ParseInt(tag[1], 10, 64)
			if err != nil {
				return 0, false
			}
			return ts, true
		}
	}
	return 0, false
}

// trackIfExpiring is the helper called from the ingest path. Safe to call
// even when the tracker hasn't been initialized (e.g. in tests).
func (db *NDB) trackIfExpiring(evt nostr.Event) {
	if db.expiration == nil {
		return
	}
	ts, ok := expirationFromTags(evt.Tags)
	if !ok {
		return
	}
	idBytes, err := hexToBytes32(evt.ID)
	if err != nil {
		return
	}
	var id32 [32]byte
	copy(id32[:], idBytes)
	db.expiration.Track(ts, id32)
}

// BootstrapExpirations scans the DB once at startup, deletes any events
// whose expiration has already passed, and populates the in-memory heap
// with the rest. Pages backwards through created_at to cover the whole
// history; each page is capped by the nostrdb query limit.
func (db *NDB) BootstrapExpirations() error {
	if db.expiration == nil {
		return nil
	}

	logger := log.GetLogger("db-expiration")
	logger.Info("Bootstrapping NIP-40 expiration heap")

	const pageSize = 5000
	now := time.Now().Unix()

	var (
		until      *time.Time
		scanned    int
		tracked    int
		expiredDel int
		pages      int
	)

	for {
		limit := pageSize
		filter := nostr.Filter{Limit: &limit}
		if until != nil {
			filter.Until = until
		}

		events, err := db.Query([]nostr.Filter{filter}, pageSize)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			break
		}
		pages++
		scanned += len(events)

		oldestTs := events[0].CreatedAt
		for _, evt := range events {
			if evt.CreatedAt < oldestTs {
				oldestTs = evt.CreatedAt
			}
			ts, ok := expirationFromTags(evt.Tags)
			if !ok {
				continue
			}
			idBytes, err := hexToBytes32(evt.ID)
			if err != nil {
				continue
			}
			var id32 [32]byte
			copy(id32[:], idBytes)

			if ts <= now {
				if err := db.DeleteNoteByID(id32); err != nil {
					logger.Warn("Failed to delete already-expired event during bootstrap",
						"event_id", evt.ID, "error", err)
					continue
				}
				expiredDel++
				continue
			}
			db.expiration.Track(ts, id32)
			tracked++
		}

		if len(events) < pageSize {
			break
		}
		// Page back: anything strictly older than the oldest we just saw.
		u := time.Unix(oldestTs-1, 0)
		until = &u
	}

	logger.Info("Bootstrap complete",
		"pages", pages,
		"events_scanned", scanned,
		"tracked", tracked,
		"already_expired_deleted", expiredDel)
	return nil
}

// RunExpirationSweeper deletes events as their expiration passes. Blocks
// until ctx is cancelled. Safe to start before BootstrapExpirations
// finishes — Track and the bootstrap-time deletes both feed the same
// heap, and the sweeper just sleeps until the first deadline arrives.
func (db *NDB) RunExpirationSweeper(ctx context.Context) {
	if db.expiration == nil {
		return
	}

	logger := log.GetLogger("db-expiration")
	logger.Info("Starting NIP-40 expiration sweeper")

	const idleSleep = 5 * time.Minute
	const minSleep = 100 * time.Millisecond

	for {
		due := db.expiration.popDue(time.Now().Unix())
		for _, item := range due {
			if err := db.DeleteNoteByID(item.id); err != nil {
				logger.Warn("Failed to delete expired event",
					"event_id", hex.EncodeToString(item.id[:]),
					"error", err)
				continue
			}
			logger.Debug("Deleted expired event",
				"event_id", hex.EncodeToString(item.id[:]))
		}

		sleep := idleSleep
		if next, ok := db.expiration.nextDeadline(); ok {
			d := time.Until(time.Unix(next, 0))
			switch {
			case d < minSleep:
				sleep = minSleep
			case d < idleSleep:
				sleep = d
			}
		}

		select {
		case <-ctx.Done():
			logger.Info("Expiration sweeper stopping")
			return
		case <-db.expiration.notify:
			// New earlier deadline pushed; loop and recompute.
		case <-time.After(sleep):
		}
	}
}
