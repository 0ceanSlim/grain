package nostrdb

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// present reports whether an event is visible to a fresh read txn.
func present(t *testing.T, db *NDB, id string) bool {
	t.Helper()
	txn, err := db.BeginQuery()
	if err != nil {
		t.Fatalf("begin query: %v", err)
	}
	defer txn.EndQuery()
	got, err := txn.GetNoteByID(id)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	return got != nil
}

// StoreEvent returns only once the event has committed, so a read right
// after it finds the event with no polling.
func TestStoreEvent_VisibleOnReturn(t *testing.T) {
	db := openTempDB(t)
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	for i := 0; i < 200; i++ {
		evt := signEvent(t, priv, pub, 1, "visible on return", nil, now-int64(i))
		if err := db.StoreEvent(context.Background(), evt); err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
		if !present(t, db, evt.ID) {
			t.Fatalf("event %d not visible right after StoreEvent returned", i)
		}
	}
}

// Two versions of one replaceable event queued back to back, from
// different goroutines, leave exactly the newer one.
func TestQueueEvent_ReplaceableBackToBack(t *testing.T) {
	db := openTempDB(t)
	now := time.Now().Unix()

	for round := 0; round < 20; round++ {
		priv, pub := newTestKey(t)
		older := signEvent(t, priv, pub, 0, `{"name":"old"}`, nil, now-10)
		newer := signEvent(t, priv, pub, 0, `{"name":"new"}`, nil, now-5)

		var wg sync.WaitGroup
		for _, evt := range []nostr.Event{newer, older} {
			wg.Add(1)
			go func(evt nostr.Event) {
				defer wg.Done()
				p, err := db.QueueEvent(context.Background(), evt)
				if err == nil {
					err = p.Wait()
				}
				if err != nil && evt.ID == newer.ID {
					t.Errorf("newer version refused: %v", err)
				}
			}(evt)
		}
		wg.Wait()

		limit := 10
		got, err := db.Query([]nostr.Filter{{Authors: []string{pub}, Kinds: []int{0}, Limit: &limit}}, limit)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if len(got) != 1 || got[0].ID != newer.ID {
			t.Fatalf("round %d: want only the newer kind 0, got %d events", round, len(got))
		}
	}
}

// A deletion queued straight after its target, before the target has
// committed, still deletes it.
func TestQueueDeletion_WaitsForQueuedTarget(t *testing.T) {
	db := openTempDB(t)
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	for i := 0; i < 50; i++ {
		target := signEvent(t, priv, pub, 1, "delete me", nil, now-10-int64(i))
		del := signEvent(t, priv, pub, 5, "", [][]string{{"e", target.ID}}, now)

		pt, err := db.QueueEvent(context.Background(), target)
		if err != nil {
			t.Fatalf("queue target: %v", err)
		}
		if err := db.ProcessDeletion(context.Background(), del); err != nil {
			t.Fatalf("deletion: %v", err)
		}
		if err := pt.Wait(); err != nil {
			t.Fatalf("target write: %v", err)
		}
		if present(t, db, target.ID) {
			t.Fatalf("round %d: target survived a deletion queued right after it", i)
		}
	}
}

// The note inside a kind-6 repost is not stored, so its author can still
// publish it.
func TestRepostContentNotIngested(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	now := time.Now().Unix()

	authorPriv, authorPub := newTestKey(t)
	note := signEvent(t, authorPriv, authorPub, 1, "original note", nil, now-60)
	raw, err := json.Marshal(note)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	reposterPriv, reposterPub := newTestKey(t)
	repost := signEvent(t, reposterPriv, reposterPub, 6, string(raw),
		[][]string{{"e", note.ID}, {"p", authorPub}}, now)
	if err := db.StoreEvent(ctx, repost); err != nil {
		t.Fatalf("store repost: %v", err)
	}
	if present(t, db, note.ID) {
		t.Fatal("the reposted note was stored though nobody published it here")
	}

	dup, err := db.CheckDuplicateEvent(note)
	if err != nil {
		t.Fatalf("duplicate check: %v", err)
	}
	if dup {
		t.Fatal("the author's own publish would be refused as a duplicate")
	}
	if err := db.StoreEvent(ctx, note); err != nil {
		t.Fatalf("store note: %v", err)
	}
	if !present(t, db, note.ID) {
		t.Fatal("the note isn't stored after its author published it")
	}
}

// Close resolves writes still in flight instead of leaving waiters hung.
func TestClose_ResolvesPending(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir, 32, 1)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	priv, pub := newTestKey(t)
	p := db.track([32]byte{1}, nil) // an id nothing will ever write
	evt := signEvent(t, priv, pub, 1, "in flight at close", nil, time.Now().Unix())
	if _, err := db.QueueEvent(context.Background(), evt); err != nil {
		t.Fatalf("queue: %v", err)
	}

	done := make(chan struct{})
	go func() {
		db.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close hung")
	}
	if err := p.Wait(); err == nil {
		t.Fatal("a write that never landed resolved without an error")
	}
}
