package userSync

import (
	"log"
	"time"

	"grain/config"
	configTypes "grain/config/types"
	"grain/server/db/mongo"
)

// Periodically triggers user sync based on config interval.
func StartPeriodicUserSync(cfg *configTypes.ServerConfig) {
	if !cfg.UserSync.UserSync {
		log.Println("User sync is disabled in the config. Skipping sync startup.")
		return
	}

	// Wait 30 seconds (for app and relay to init) before the initial sync
	//log.Println("Waiting 30 seconds before starting initial user sync...")
	time.Sleep(30 * time.Second)

	// Run the first sync
	log.Println("Running initial user sync...")
	runUserSync(cfg)

	// Start periodic sync if interval is set
	interval := cfg.UserSync.Interval
	if interval <= 0 {
		log.Println("User sync interval is not set. Skipping periodic sync.")
		return
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		runUserSync(cfg)
	}
}

// Runs user sync for all relevant authors.
func runUserSync(cfg *configTypes.ServerConfig) {
	authors := mongo.GetAllAuthorsFromRelay(cfg)
	if cfg.UserSync.ExcludeNonWhitelisted {
		authors = filterWhitelistedAuthors(authors)
	}

	for _, author := range authors {
		go triggerUserSync(author, &cfg.UserSync, cfg) // Run sync concurrently
	}
}

// Filters only whitelisted authors.
func filterWhitelistedAuthors(authors []string) []string {
	filtered := []string{}
	for _, author := range authors {
		if config.IsPubKeyWhitelisted(author, true) {
			filtered = append(filtered, author)
		}
	}
	return filtered
}
