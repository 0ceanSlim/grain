package cache

import (
	"sync"
	"time"
)

// UserCache manages in-memory user data with automatic expiration
type UserCache struct {
	mu     sync.RWMutex
	data   map[string]CachedUserData
	expiry time.Duration
}

// CachedUserData holds user metadata and mailbox information
type CachedUserData struct {
	Metadata  string    // JSON serialized metadata (kind 0 event)
	Mailboxes string    // JSON serialized mailboxes (kind 10002 relay list)
	Timestamp time.Time // Time of insertion for expiration
}

// Global cache instance with 10 minute expiry
var cache = &UserCache{
	data:   make(map[string]CachedUserData),
	expiry: 10 * time.Minute,
}

// SetUserData stores user metadata and mailbox data in cache
func SetUserData(publicKey string, metadata, mailboxes string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	
	cache.data[publicKey] = CachedUserData{
		Metadata:  metadata,
		Mailboxes: mailboxes,
		Timestamp: time.Now(),
	}
}

// GetUserData retrieves cached user data if not expired
func GetUserData(publicKey string) (CachedUserData, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	data, exists := cache.data[publicKey]
	if !exists || time.Since(data.Timestamp) > cache.expiry {
		return CachedUserData{}, false
	}
	
	return data, true
}

// ClearUserData removes a specific user from cache
func ClearUserData(publicKey string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	
	delete(cache.data, publicKey)
}

// CleanupExpired removes all expired entries from cache
func CleanupExpired() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := time.Now()
	for key, data := range cache.data {
		if now.Sub(data.Timestamp) > cache.expiry {
			delete(cache.data, key)
		}
	}
}