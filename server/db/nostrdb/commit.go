package nostrdb

/*
#include "nostrdb.h"
*/
import "C"
import (
	"errors"
	"hash/fnv"
	"sort"
	"sync"
	"time"
	"unsafe"
)

// A relay may only acknowledge a write once it is durable, but
// ndb_process_event just queues the event for nostrdb's ingester and writer
// threads. So every queued write is registered here as pending, and one
// resolver looks the pending ids up each time the writer finishes a batch:
// a note visible to a read txn has committed. Callers get a PendingWrite
// and send OK only after Wait returns.

// commitTimeout bounds how long a write may stay queued before grain gives
// up on it and tells the client so. A healthy writer commits in
// milliseconds; this only trips when it has stalled or refused the note.
const commitTimeout = 10 * time.Second

// resolveInterval is the fallback check for writes no batch will report:
// a note nostrdb already held, or one its ingester dropped.
const resolveInterval = 25 * time.Millisecond

// ErrNotCommitted means a queued write didn't become visible in time.
var ErrNotCommitted = errors.New("error: the write was not confirmed in time; try again")

var errClosing = errors.New("error: relay is shutting down")

// PendingWrite is a queued write that isn't known to have committed yet.
// A nil *PendingWrite stands for a write with nothing to wait for (an
// ephemeral event, say) and Wait returns nil at once.
type PendingWrite struct {
	id       [32]byte
	deadline time.Time
	release  func()
	done     chan struct{}
	err      error
}

// Wait blocks until the write has committed, or returns ErrNotCommitted.
func (p *PendingWrite) Wait() error {
	if p == nil {
		return nil
	}
	<-p.done
	return p.err
}

type commitTracker struct {
	mu      sync.Mutex
	pending map[[32]byte][]*PendingWrite
	kick    chan struct{}
	stop    chan struct{}
	wg      sync.WaitGroup
	keys    keyLocks
}

// startCommitTracker launches the epoch watcher and the resolver. It is
// called once, right after ndb_init.
func (db *NDB) startCommitTracker() {
	t := &db.commits
	t.pending = make(map[[32]byte][]*PendingWrite)
	t.kick = make(chan struct{}, 1)
	t.stop = make(chan struct{})

	ndb := db.ndb
	t.wg.Add(2)
	go func() {
		defer t.wg.Done()
		epoch := C.ndb_commit_epoch(ndb)
		for {
			next := C.ndb_wait_for_commit(ndb, epoch)
			if next == 0 {
				return
			}
			epoch = next
			select {
			case t.kick <- struct{}{}:
			default:
			}
		}
	}()
	go func() {
		defer t.wg.Done()
		tick := time.NewTicker(resolveInterval)
		defer tick.Stop()
		for {
			select {
			case <-t.stop:
				t.failAll(errClosing)
				return
			case <-t.kick:
			case <-tick.C:
			}
			db.resolvePending(time.Now())
		}
	}()
}

// stopCommitTracker ends both goroutines; it must run before ndb_destroy,
// since the watcher is blocked inside nostrdb.
func (db *NDB) stopCommitTracker() {
	t := &db.commits
	if t.stop == nil {
		return
	}
	C.ndb_wake_commit_waiters(db.ndb)
	close(t.stop)
	t.wg.Wait()
	t.stop = nil
}

// track registers a write that is about to be queued. release, if not nil,
// runs once the write resolves either way.
func (db *NDB) track(id [32]byte, release func()) *PendingWrite {
	p := &PendingWrite{
		id:       id,
		deadline: time.Now().Add(commitTimeout),
		release:  release,
		done:     make(chan struct{}),
	}
	t := &db.commits
	t.mu.Lock()
	t.pending[id] = append(t.pending[id], p)
	t.mu.Unlock()
	return p
}

// untrack drops a write that never made it into nostrdb's queue.
func (db *NDB) untrack(p *PendingWrite, err error) {
	t := &db.commits
	t.mu.Lock()
	list := t.pending[p.id]
	for i, q := range list {
		if q == p {
			list = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(t.pending, p.id)
	} else {
		t.pending[p.id] = list
	}
	t.mu.Unlock()
	p.finish(err)
}

// awaitPending waits for any queued write of this id to resolve, so a
// deletion sees a target the same client sent just before it.
func (db *NDB) awaitPending(id [32]byte) {
	t := &db.commits
	t.mu.Lock()
	list := append([]*PendingWrite(nil), t.pending[id]...)
	t.mu.Unlock()
	for _, p := range list {
		p.Wait()
	}
}

func (p *PendingWrite) finish(err error) {
	p.err = err
	if p.release != nil {
		p.release()
	}
	close(p.done)
}

// resolvePending finishes every pending write that is now visible, and
// fails those past their deadline.
func (db *NDB) resolvePending(now time.Time) {
	t := &db.commits
	t.mu.Lock()
	if len(t.pending) == 0 {
		t.mu.Unlock()
		return
	}
	ids := make([][32]byte, 0, len(t.pending))
	for id := range t.pending {
		ids = append(ids, id)
	}
	t.mu.Unlock()

	visible := make(map[[32]byte]bool, len(ids))
	txn, err := db.BeginQuery()
	if err != nil {
		return
	}
	for _, id := range ids {
		var size C.size_t
		var key C.uint64_t
		note := C.ndb_get_note_by_id(&txn.txn, (*C.uchar)(unsafe.Pointer(&id[0])), &size, &key)
		visible[id] = note != nil
	}
	txn.EndQuery()

	var done, expired []*PendingWrite
	t.mu.Lock()
	for _, id := range ids {
		list := t.pending[id]
		if visible[id] {
			done = append(done, list...)
			delete(t.pending, id)
			continue
		}
		keep := list[:0]
		for _, p := range list {
			if now.After(p.deadline) {
				expired = append(expired, p)
			} else {
				keep = append(keep, p)
			}
		}
		if len(keep) == 0 {
			delete(t.pending, id)
		} else {
			t.pending[id] = keep
		}
	}
	t.mu.Unlock()

	for _, p := range done {
		p.finish(nil)
	}
	for _, p := range expired {
		p.finish(ErrNotCommitted)
	}
}

func (t *commitTracker) failAll(err error) {
	t.mu.Lock()
	var all []*PendingWrite
	for id, list := range t.pending {
		all = append(all, list...)
		delete(t.pending, id)
	}
	t.mu.Unlock()
	for _, p := range all {
		p.finish(err)
	}
}

// keyLocks serializes writes to one replaceable or addressable address.
// A lock is held from the newer-version check until the write commits, so
// two versions sent back to back can't both pass the check. Addresses are
// striped over a fixed set of mutexes; a collision only costs a short wait.
type keyLocks struct {
	stripes [256]sync.Mutex
}

func stripeOf(addr string) int {
	h := fnv.New32a()
	h.Write([]byte(addr))
	return int(h.Sum32() % 256)
}

// lock takes the lock for one address and returns its release.
func (k *keyLocks) lock(addr string) func() {
	return k.lockAll([]string{addr})
}

// lockAll takes the locks for several addresses, in stripe order so two
// callers can't deadlock, and returns one release for all of them.
func (k *keyLocks) lockAll(addrs []string) func() {
	seen := make(map[int]bool, len(addrs))
	var idx []int
	for _, a := range addrs {
		s := stripeOf(a)
		if !seen[s] {
			seen[s] = true
			idx = append(idx, s)
		}
	}
	sort.Ints(idx)
	for _, s := range idx {
		k.stripes[s].Lock()
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for i := len(idx) - 1; i >= 0; i-- {
				k.stripes[idx[i]].Unlock()
			}
		})
	}
}
