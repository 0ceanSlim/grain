package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Purge tuning. These bound each run so a large backlog drains over several
// passes instead of one marathon (or, as in the RC, never): the old
// implementation issued a single {until, limit} query, which nostrdb answers in
// storage order — so it re-scanned the same head every run and never reached the
// backlog behind it.
const (
	// purgeKindSampleLimit bounds the broad query used to discover which kinds
	// are present when purging by category. Common kinds always surface in a
	// sample this size; a rare kind missed one run is caught the next.
	purgeKindSampleLimit = 50000
	// purgeRunBudget caps deletes per run so the DB trends to steady state
	// rather than accumulating a full retention window of backlog each pass.
	purgeRunBudget = 100000
	// purgePageSize is the per-kind page walked through the kind index.
	purgePageSize = 500
)

// PurgeOldEvents removes events older than the configured retention window.
// Whitelisted pubkeys (configured members) are excluded when ExcludeWhitelisted
// is set — the "non-member cleanup" knob that keeps member content forever while
// aging out drive-by events.
//
// It pages per kind through nostrdb's kind index, which is ordered by
// created_at, so it actually drains the backlog rather than re-scanning the same
// storage-order head each run. Deletes are capped at purgeRunBudget per run.
func (db *NDB) PurgeOldEvents(cfg *cfgType.EventPurgeConfig, whitelistedPubkeys []string) int {
	if !cfg.Enabled {
		log.DBPurge().Debug("Event purging is disabled")
		return 0
	}

	cutoff := time.Now().Unix() - int64(cfg.KeepIntervalHours*3600)
	log.DBPurge().Info("Starting event purge",
		"keep_hours", cfg.KeepIntervalHours,
		"cutoff_time", time.Unix(cutoff, 0).Format(time.RFC3339))

	whitelistSet := make(map[string]bool, len(whitelistedPubkeys))
	for _, pk := range whitelistedPubkeys {
		whitelistSet[pk] = true
	}
	keepKinds := make(map[int]bool, len(cfg.KeepKinds))
	for _, k := range cfg.KeepKinds {
		keepKinds[k] = true
	}

	// Which kinds to walk: the explicit list in kind mode, else every kind
	// actually present in the DB (category mode spans whole ranges, so the kind
	// set can't be enumerated statically).
	var kinds []int
	if cfg.PurgeByKindEnabled && len(cfg.KindsToPurge) > 0 {
		kinds = cfg.KindsToPurge
	} else {
		kinds = db.distinctKinds(purgeKindSampleLimit)
	}

	deleted, failed, keptKinds, catKinds := 0, 0, 0, 0
	for _, k := range kinds {
		if deleted >= purgeRunBudget {
			break
		}
		if keepKinds[k] {
			keptKinds++
			continue
		}
		// The purge_by_category gate applies in both modes: when the map is
		// configured, a kind's category must be explicitly enabled. Checked once
		// per kind since every event of a kind shares its category.
		if len(cfg.PurgeByCategory) > 0 && !categoryPermitsPurge(k, cfg.PurgeByCategory) {
			catKinds++
			continue
		}
		d, f := db.purgeKind(k, cutoff, whitelistSet, cfg.ExcludeWhitelisted, purgeRunBudget-deleted)
		deleted += d
		failed += f
	}

	log.DBPurge().Info("Purge completed",
		"deleted", deleted,
		"failed", failed,
		"kinds_considered", len(kinds),
		"kinds_skipped_keeplist", keptKinds,
		"kinds_skipped_category", catKinds,
		"budget_hit", deleted >= purgeRunBudget)

	return deleted
}

// distinctKinds samples the DB and returns the distinct event kinds present, so
// category-mode purging knows which kinds to walk without iterating every
// possible kind number.
func (db *NDB) distinctKinds(sampleLimit int) []int {
	events, err := db.Query([]nostr.Filter{{Limit: &sampleLimit}}, sampleLimit)
	if err != nil {
		log.DBPurge().Error("Failed to sample kinds for purge", "error", err)
		return nil
	}
	set := make(map[int]struct{})
	for _, e := range events {
		set[e.Kind] = struct{}{}
	}
	kinds := make([]int, 0, len(set))
	for k := range set {
		kinds = append(kinds, k)
	}
	return kinds
}

// purgeKind walks one kind's index from cutoff backwards, deleting
// non-whitelisted events until the kind is drained or the budget is spent.
// Paging is created_at-ordered (the kind index), so it reaches the whole
// backlog. Returns (deleted, failed).
func (db *NDB) purgeKind(kind int, cutoff int64, whitelistSet map[string]bool, excludeWhitelisted bool, budget int) (int, int) {
	if budget <= 0 {
		return 0, 0
	}
	deleted, failed := 0, 0
	until := cutoff

	for deleted < budget {
		lim := purgePageSize
		untilTime := time.Unix(until, 0)
		events, err := db.Query([]nostr.Filter{{
			Kinds: []int{kind},
			Until: &untilTime,
			Limit: &lim,
		}}, purgePageSize)
		if err != nil {
			log.DBPurge().Warn("Purge query failed for kind", "kind", kind, "error", err)
			break
		}
		if len(events) == 0 {
			break
		}

		oldest := until
		del := 0
		for _, evt := range events {
			if evt.CreatedAt < oldest {
				oldest = evt.CreatedAt
			}
			if excludeWhitelisted && whitelistSet[evt.PubKey] {
				continue
			}
			idBytes, err := hexToBytes32(evt.ID)
			if err != nil {
				failed++
				continue
			}
			var id32 [32]byte
			copy(id32[:], idBytes)
			if err := db.DeleteNoteByID(id32); err != nil {
				failed++
				continue
			}
			deleted++
			del++
			if deleted >= budget {
				break
			}
		}

		// Advance the cursor. When we deleted from this window, re-query at the
		// same oldest (inclusive): the deleted events are gone so the set shrinks
		// and same-second remainders are still reached — this terminates because
		// each pass removes at least one event <= oldest. When we deleted nothing
		// (a boundary of only-whitelisted events), step strictly past it.
		if del > 0 {
			until = oldest
		} else {
			until = oldest - 1
		}
	}
	return deleted, failed
}

// ScheduleEventPurging runs periodic event purging at the configured interval.
// It returns when ctx is cancelled so the loop doesn't outlive the server
// instance (and its now-closed DB handle) on a config-reload restart (#93).
func (db *NDB) ScheduleEventPurging(ctx context.Context, cfg *cfgType.ServerConfig, getWhitelistedPubkeys func() []string) {
	if !cfg.EventPurge.Enabled {
		log.DBPurge().Info("Event purging is disabled in configuration")
		return
	}

	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalMinutes) * time.Minute
	log.DBPurge().Info("Starting scheduled event purging",
		"interval_minutes", cfg.EventPurge.PurgeIntervalMinutes,
		"keep_hours", cfg.EventPurge.KeepIntervalHours)

	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	// Run initial purge if not disabled
	if !cfg.EventPurge.DisableAtStartup {
		log.DBPurge().Info("Running initial purge at startup")
		db.PurgeOldEvents(&cfg.EventPurge, getWhitelistedPubkeys())
	}

	for {
		select {
		case <-ctx.Done():
			log.DBPurge().Info("Event purging stopping")
			return
		case <-ticker.C:
			log.DBPurge().Info("Running scheduled purge")
			purged := db.PurgeOldEvents(&cfg.EventPurge, getWhitelistedPubkeys())
			log.DBPurge().Info("Scheduled purging completed", "purged", purged)
		}
	}
}

// purgeCategoryForKind returns the v0.4-compatible category name used by
// the `purge_by_category` config map. Names match the v0.4 MongoDB-era
// server/utils/determineEventCategory.go exactly so pre-existing operator
// configs keep working unchanged.
func purgeCategoryForKind(kind int) string {
	switch {
	case kind == 0, kind == 3, kind >= 10000 && kind < 20000:
		return "replaceable"
	case kind == 1, kind >= 4 && kind < 45, kind >= 1000 && kind < 10000:
		return "regular"
	case kind == 2:
		return "deprecated"
	case kind >= 20000 && kind < 30000:
		return "ephemeral"
	case kind >= 30000 && kind < 40000:
		return "parameterized_replaceable"
	default:
		return "unknown"
	}
}

// categoryPermitsPurge looks an event up in the configured
// purge_by_category map and returns true when the event's category is
// explicitly enabled. Accepts "addressable" as a v0.5 alias for
// "parameterized_replaceable" so configs using either name work.
func categoryPermitsPurge(kind int, m map[string]bool) bool {
	cat := purgeCategoryForKind(kind)
	if v, ok := m[cat]; ok {
		return v
	}
	// Alias: accept "addressable" <-> "parameterized_replaceable".
	if cat == "parameterized_replaceable" {
		if v, ok := m["addressable"]; ok {
			return v
		}
	}
	if cat == "addressable" {
		if v, ok := m["parameterized_replaceable"]; ok {
			return v
		}
	}
	return false
}
