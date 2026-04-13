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
