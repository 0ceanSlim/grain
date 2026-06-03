package cache

import (
	"context"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// StartCacheCleanup starts a background goroutine to clean up expired cache
// entries. It exits when ctx is cancelled so it doesn't outlive the server
// instance that started it (#93).
func StartCacheCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Minute) // Clean up every 15 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				CleanupExpired()
				log.ClientCache().Debug("Cache cleanup completed")
			}
		}
	}()

	log.ClientCache().Debug("Cache cleanup routine started")
}
