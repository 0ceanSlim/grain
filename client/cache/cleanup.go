package cache

import (
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// startCacheCleanup starts a background goroutine to clean up expired cache entries
func StartCacheCleanup() {
	go func() {
		ticker := time.NewTicker(15 * time.Minute) // Clean up every 15 minutes
		defer ticker.Stop()

		for range ticker.C {
			CleanupExpired()
			log.ClientCache().Debug("Cache cleanup completed")
		}
	}()
	
	log.ClientCache().Debug("Cache cleanup routine started")
}
