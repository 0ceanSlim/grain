package userSync

import (
	"time"

	"github.com/0ceanslim/grain/config"
	configTypes "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/db/mongo"
)

// StartPeriodicUserSync periodically triggers user sync based on config interval
func StartPeriodicUserSync(cfg *configTypes.ServerConfig) {
	if !cfg.UserSync.UserSync {
		syncLog().Debug("User sync is disabled in the config. Skipping sync startup.")
		return
	}

	if cfg.UserSync.DisableAtStartup {
		syncLog().Debug("User sync is disabled at startup. Skipping initial sync.")
	} else {
		time.Sleep(30 * time.Second) // Wait before initial sync
		syncLog().Info("Running initial user sync...")
		runUserSync(cfg)
	}

	interval := cfg.UserSync.Interval
	if interval <= 0 {
		syncLog().Warn("User sync interval is not set. Skipping periodic sync.")
		return
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	syncLog().Info("Started periodic user sync", "interval_minutes", interval)

	for {
		<-ticker.C
		runUserSync(cfg)
	}
}

// runUserSync runs user sync for all relevant authors
func runUserSync(cfg *configTypes.ServerConfig) {
	syncLog().Info("Starting periodic user sync run")
	
	authors := mongo.GetAllAuthorsFromRelay(cfg)
	syncLog().Debug("Retrieved authors from relay", "total_authors", len(authors))

	// Filter authors if required using cache (ignores enabled state for sync operations)
	if cfg.UserSync.ExcludeNonWhitelisted {
		authors = filterWhitelistedAuthorsCached(authors)
		syncLog().Info("Filtered authors for sync", 
			"exclude_non_whitelisted", true,
			"filtered_authors", len(authors))
	}

	successCount := 0
	for _, author := range authors {
		syncLog().Debug("Starting user sync for author", "pubkey", author)
		
		triggerUserSync(author, &cfg.UserSync, cfg)
		successCount++
		
		// Small delay between each author's sync to reduce load
		time.Sleep(2 * time.Second)
	}
	
	syncLog().Info("Periodic user sync run completed", 
		"total_authors", len(authors),
		"synced_authors", successCount)
}

// filterWhitelistedAuthorsCached uses cache for filtering (ignores enabled state for sync)
func filterWhitelistedAuthorsCached(authors []string) []string {
	pubkeyCache := config.GetPubkeyCache()
	filtered := make([]string, 0, len(authors))
	
	for _, author := range authors {
		// Use IsWhitelisted (not IsWhitelistedForValidation) to ignore enabled state
		if pubkeyCache.IsWhitelisted(author) {
			filtered = append(filtered, author)
		}
	}
	
	syncLog().Debug("Filtered authors using cache", 
		"total_authors", len(authors),
		"whitelisted_authors", len(filtered))
	
	return filtered
}