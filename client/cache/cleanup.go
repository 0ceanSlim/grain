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

		for {
			select {
			case <-ticker.C:
				CleanupExpired()
				log.Util().Debug("Cache cleanup completed")
			}
		}
	}()
	
	log.Util().Debug("Cache cleanup routine started")
}
