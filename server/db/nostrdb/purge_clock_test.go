package nostrdb

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/db/arrivals"
)

func purgeCfg(clock string) *cfgType.EventPurgeConfig {
	return &cfgType.EventPurgeConfig{
		Enabled:           true,
		KeepIntervalHours: 1,
		RetentionClock:    clock,
	}
}

// An event received 5h after it was written is past a 3h late-arrival
// threshold, so the received clock ages it from now and a 1h keep window
// keeps it. One received 2h late is under the threshold and is aged from
// created_at. Switching to the created_at clock purges the late one too.
func TestPurge_RetentionClock(t *testing.T) {
	db := openTempDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ledgerPath := filepath.Join(t.TempDir(), "arrivals.log")
	if err := db.TrackArrivals(ctx, ledgerPath, 3*time.Hour); err != nil {
		t.Fatal(err)
	}

	priv, pub := newTestKey(t)
	now := time.Now().Unix()
	late := signEvent(t, priv, pub, 1, "late arrival", nil, now-5*3600)
	prompt := signEvent(t, priv, pub, 1, "under the threshold", nil, now-2*3600)
	if err := db.StoreEvent(ctx, late); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreEvent(ctx, prompt); err != nil {
		t.Fatal(err)
	}
	waitForIngest(t, db, late.ID, true)
	waitForIngest(t, db, prompt.ID, true)

	if _, ok := db.arrivals.Load().ArrivedAt(late.ID); !ok {
		t.Fatal("late arrival was not recorded")
	}
	if _, ok := db.arrivals.Load().ArrivedAt(prompt.ID); ok {
		t.Fatal("arrival under the threshold was recorded")
	}

	db.PurgeOldEvents(purgeCfg(cfgType.RetentionClockReceived), nil)
	waitForIngest(t, db, prompt.ID, false)
	time.Sleep(200 * time.Millisecond)
	waitForIngest(t, db, late.ID, true)

	db.PurgeOldEvents(purgeCfg(cfgType.RetentionClockCreatedAt), nil)
	waitForIngest(t, db, late.ID, false)

	// The entry still protects (it arrived after the cutoff), and Close
	// saves it.
	db.Close()
	saved, err := arrivals.Open(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.ArrivedAt(late.ID); !ok {
		t.Fatal("ledger entry was not saved on Close")
	}
}

// The kind scan must find a kind whose events are all old. The newest-N
// sample it replaces could not.
func TestKindsWithEventsBefore(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	old := signEvent(t, priv, pub, 7777, "old", nil, now-3*3600)
	fresh := signEvent(t, priv, pub, 8888, "fresh", nil, now)
	if err := db.StoreEvent(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreEvent(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	waitForIngest(t, db, old.ID, true)
	waitForIngest(t, db, fresh.ID, true)

	started := time.Now()
	kinds := db.kindsWithEventsBefore(now - 3600)
	t.Logf("kind scan took %v", time.Since(started))
	if !slices.Contains(kinds, 7777) {
		t.Errorf("kinds %v missing 7777, which only has an old event", kinds)
	}
	if slices.Contains(kinds, 8888) {
		t.Errorf("kinds %v include 8888, which has no event before the cutoff", kinds)
	}
}
