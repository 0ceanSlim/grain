package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PurgeOldEvents removes events older than the configured retention window.
// Whitelisted pubkeys (configured members) are excluded when
// ExcludeWhitelisted is set — this is the "non-member cleanup" knob that
// keeps member content forever while aging out drive-by events.
func (db *NDB) PurgeOldEvents(cfg *cfgType.EventPurgeConfig, whitelistedPubkeys []string) int {
	if !cfg.Enabled {
		log.GetLogger("db-purge").Debug("Event purging is disabled")
		return 0
	}

	currentTime := time.Now().Unix()
	cutoff := currentTime - int64(cfg.KeepIntervalHours*3600)
	cutoffTime := time.Unix(cutoff, 0)

	log.GetLogger("db-purge").Info("Starting event purge",
		"keep_hours", cfg.KeepIntervalHours,
		"cutoff_time", cutoffTime.Format(time.RFC3339))

	// Build whitelist exclusion set for fast lookup
	whitelistSet := make(map[string]bool, len(whitelistedPubkeys))
	for _, pk := range whitelistedPubkeys {
		whitelistSet[pk] = true
	}

	// Query old events using until filter
	untilTime := time.Unix(cutoff, 0)
	limit := 5000

	filters := []nostr.Filter{{
		Until: &untilTime,
		Limit: &limit,
	}}

	// If purging by specific kinds, add kind filter
	if cfg.PurgeByKindEnabled && len(cfg.KindsToPurge) > 0 {
		filters[0].Kinds = cfg.KindsToPurge
	}

	events, err := db.Query(filters, limit)
	if err != nil {
		log.GetLogger("db-purge").Error("Failed to query events for purging", "error", err)
		return 0
	}

	purgeCount := 0
	failCount := 0
	for _, evt := range events {
		// Skip whitelisted ("member") pubkeys.
		if cfg.ExcludeWhitelisted && whitelistSet[evt.PubKey] {
			continue
		}

		// v0.4 purge_by_category gate: when the map is configured, an
		// event's category must resolve to an explicit `true` entry or
		// it's kept. This is the behavior v0.4 operators rely on to
		// purge only some categories (e.g. regular + deprecated) while
		// keeping replaceable profiles forever. When the map is empty
		// or nil, the gate is inactive and every candidate that made it
		// past the cutoff/kind/whitelist filters is deleted.
		if len(cfg.PurgeByCategory) > 0 {
			if !categoryPermitsPurge(evt.Kind, cfg.PurgeByCategory) {
				continue
			}
		}

		idBytes, err := hexToBytes32(evt.ID)
		if err != nil {
			log.GetLogger("db-purge").Warn("Skipping malformed event id",
				"event_id", evt.ID, "error", err)
			failCount++
			continue
		}
		var id32 [32]byte
		copy(id32[:], idBytes)
		if err := db.DeleteNoteByID(id32); err != nil {
			log.GetLogger("db-purge").Error("Delete failed during purge",
				"event_id", evt.ID, "error", err)
			failCount++
			continue
		}
		purgeCount++
	}

	log.GetLogger("db-purge").Info("Purge completed",
		"events_scanned", len(events),
		"deleted", purgeCount,
		"failed", failCount)

	return purgeCount
}

// ScheduleEventPurging runs periodic event purging at the configured interval.
func (db *NDB) ScheduleEventPurging(cfg *cfgType.ServerConfig, getWhitelistedPubkeys func() []string) {
	if !cfg.EventPurge.Enabled {
		log.GetLogger("db-purge").Info("Event purging is disabled in configuration")
		return
	}

	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalMinutes) * time.Minute
	log.GetLogger("db-purge").Info("Starting scheduled event purging",
		"interval_minutes", cfg.EventPurge.PurgeIntervalMinutes,
		"keep_hours", cfg.EventPurge.KeepIntervalHours)

	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	// Run initial purge if not disabled
	if !cfg.EventPurge.DisableAtStartup {
		log.GetLogger("db-purge").Info("Running initial purge at startup")
		db.PurgeOldEvents(&cfg.EventPurge, getWhitelistedPubkeys())
	}

	for range ticker.C {
		log.GetLogger("db-purge").Info("Running scheduled purge")
		purged := db.PurgeOldEvents(&cfg.EventPurge, getWhitelistedPubkeys())
		log.GetLogger("db-purge").Info("Scheduled purging completed", "purged", purged)
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
