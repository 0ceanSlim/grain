// client/cache/cache.go
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

// Global cache instance with 1 hour expiry (increased from 10 minutes)
// User profile data doesn't change frequently, so longer cache is reasonable
var cache = &UserCache{
	data:   make(map[string]CachedUserData),
	expiry: 60 * time.Minute, // Increased to 1 hour
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

// GetUserDataWithAge retrieves cached user data with age information
// Useful for determining if refresh is needed soon
func GetUserDataWithAge(publicKey string) (CachedUserData, time.Duration, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	data, exists := cache.data[publicKey]
	if !exists {
		return CachedUserData{}, 0, false
	}
	
	age := time.Since(data.Timestamp)
	if age > cache.expiry {
		return CachedUserData{}, age, false
	}
	
	return data, age, true
}

// IsExpiringSoon checks if cache will expire within the given duration
func IsExpiringSoon(publicKey string, within time.Duration) bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	data, exists := cache.data[publicKey]
	if !exists {
		return true // No data means it needs refresh
	}
	
	age := time.Since(data.Timestamp)
	return (cache.expiry - age) <= within
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

// SetCacheExpiry allows dynamic configuration of cache expiry
func SetCacheExpiry(duration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.expiry = duration
}

// GetCacheExpiry returns the current cache expiry duration
func GetCacheExpiry() time.Duration {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.expiry
}