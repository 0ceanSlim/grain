package userSync

import (
	"log"
	"time"

	"github.com/0ceanslim/grain/config"
	configTypes "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/db/mongo"
)

// Periodically triggers user sync based on config interval.
func StartPeriodicUserSync(cfg *configTypes.ServerConfig) {
	if !cfg.UserSync.UserSync {
		syncLog().Debug("User sync is disabled in the config. Skipping sync startup.")
		return
	}

	if cfg.UserSync.DisableAtStartup {
		syncLog().Debug("User sync is disabled at startup. Skipping initial sync.")
	} else {
		time.Sleep(30 * time.Second) // Wait before initial sync
		log.Println("Running initial user sync...")
		runUserSync(cfg)
	}

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
//func runUserSync(cfg *configTypes.ServerConfig) {
//	authors := mongo.GetAllAuthorsFromRelay(cfg)
//
//	// Filter authors if required
//	if cfg.UserSync.ExcludeNonWhitelisted {
//		authors = filterWhitelistedAuthors(authors)
//	}
//
//	for _, author := range authors {
//		log.Printf("Starting user sync for author: %s", author)
//
//		// Run sync sequentially (removes concurrency to avoid rate limiting)
//		triggerUserSync(author, &cfg.UserSync, cfg)
//
//		// Optional: Small delay between each author's sync to further reduce load
//		time.Sleep(2 * time.Second)
//	}
//}
func runUserSync(cfg *configTypes.ServerConfig) {
	authors := mongo.GetAllAuthorsFromRelay(cfg)

	// Filter authors if required using cache
	if cfg.UserSync.ExcludeNonWhitelisted {
		authors = filterWhitelistedAuthorsCached(authors)
	}

	for _, author := range authors {
		log.Printf("Starting user sync for author: %s", author)
		triggerUserSync(author, &cfg.UserSync, cfg)
		time.Sleep(2 * time.Second)
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
