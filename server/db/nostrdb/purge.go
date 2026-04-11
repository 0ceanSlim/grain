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

// PurgeOldEvents removes events older than the configured interval.
// Since nostrdb doesn't support direct deletion, this queries for old events
// and logs what would be purged. True purging requires nostrdb delete support.
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
	for _, evt := range events {
		// Skip whitelisted pubkeys
		if cfg.ExcludeWhitelisted && whitelistSet[evt.PubKey] {
			continue
		}
		// TODO: Actually delete/flag the event when nostrdb supports it
		purgeCount++
	}

	log.GetLogger("db-purge").Info("Purge scan completed",
		"events_found", len(events),
		"eligible_for_purge", purgeCount)

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
