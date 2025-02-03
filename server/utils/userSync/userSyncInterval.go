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
	interval := cfg.UserSync.Interval
	if interval <= 0 {
		log.Println("User sync interval is not set. Skipping periodic sync.")
		return
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		authors := mongo.GetAllAuthorsFromRelay(cfg)
		if cfg.UserSync.ExcludeNonWhitelisted {
			authors = filterWhitelistedAuthors(authors)
		}
		for _, author := range authors {
			go triggerUserSync(author, &cfg.UserSync, cfg)
		}
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
