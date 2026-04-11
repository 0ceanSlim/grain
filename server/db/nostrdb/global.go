package nostrdb

import (
	"sync"

	"github.com/0ceanslim/grain/server/utils/log"
)

var (
	globalDB   *NDB
	globalOnce sync.Once
	globalMu   sync.RWMutex
)

// SetGlobalDB sets the global nostrdb instance.
func SetGlobalDB(db *NDB) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalDB = db
	log.GetLogger("db").Info("Global nostrdb instance set")
}

// GetDB returns the global nostrdb instance.
// Returns nil if the database hasn't been initialized.
func GetDB() *NDB {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalDB
}

// IsAvailable returns true if the database is initialized and ready.
func IsAvailable() bool {
	return GetDB() != nil
}
