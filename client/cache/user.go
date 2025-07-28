package cache

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// UserCache manages in-memory user data with automatic expiration
type UserCache struct {
	mu           sync.RWMutex
	data         map[string]CachedUserData
	clientRelays map[string][]string // publicKey -> []relayJSON strings
	expiry       time.Duration
}

// CachedUserData holds user metadata and mailbox information
type CachedUserData struct {
	Metadata  string    `json:"metadata"`  // JSON serialized metadata (kind 0 event)
	Mailboxes string    `json:"mailboxes"` // JSON serialized mailboxes (kind 10002 relay list)
	Timestamp time.Time `json:"timestamp"` // Time of insertion for expiration
}

// ClientRelayConfig represents relay configuration with permissions
type ClientRelayConfig struct {
	URL       string    `json:"url"`
	Read      bool      `json:"read"`
	Write     bool      `json:"write"`
	Connected bool      `json:"connected"` // Whether the client is connected to this relay
	AddedAt   time.Time `json:"added_at"`  // Time when the relay was added
}

// Global cache instance with 1 hour expiry
var cache = &UserCache{
	data:         make(map[string]CachedUserData),
	clientRelays: make(map[string][]string),
	expiry:       60 * time.Minute,
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
		return true
	}

	age := time.Since(data.Timestamp)
	return (cache.expiry - age) <= within
}

// ClearUserData removes a specific user from cache
func ClearUserData(publicKey string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	delete(cache.data, publicKey)
	delete(cache.clientRelays, publicKey)
}

// CleanupExpired removes all expired entries from cache
func CleanupExpired() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := time.Now()
	for key, data := range cache.data {
		if now.Sub(data.Timestamp) > cache.expiry {
			delete(cache.data, key)
			delete(cache.clientRelays, key)
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

// Legacy function for backward compatibility
func AddClientRelay(publicKey, relayURL string) error {
	return AddClientRelayWithPermissions(publicKey, relayURL, true, true)
}

// GetClientRelays returns user's cached client relays (legacy format)
func GetClientRelays(publicKey string) []string {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	relayStrings, exists := cache.clientRelays[publicKey]
	if !exists {
		return nil
	}

	// Extract URLs from JSON strings for legacy compatibility
	var urls []string
	for _, relayString := range relayStrings {
		var relayInfo ClientRelayConfig
		if err := json.Unmarshal([]byte(relayString), &relayInfo); err != nil {
			// Handle legacy format (plain URL strings)
			urls = append(urls, relayString)
		} else {
			urls = append(urls, relayInfo.URL)
		}
	}

	return urls
}

// CacheUserDataFromObjects caches user metadata and mailboxes from Go objects
func CacheUserDataFromObjects(publicKey string, metadata *nostr.Event, mailboxes *core.Mailboxes) {
	var metadataStr, mailboxesStr string

	if metadata != nil {
		if metadataBytes, err := json.Marshal(metadata); err == nil {
			metadataStr = string(metadataBytes)
		} else {
			log.ClientCache().Warn("Failed to marshal metadata for cache", "pubkey", publicKey, "error", err)
		}
	}

	if mailboxes != nil {
		if mailboxBytes, err := json.Marshal(mailboxes); err == nil {
			mailboxesStr = string(mailboxBytes)
		} else {
			log.ClientCache().Warn("Failed to marshal mailboxes for cache", "pubkey", publicKey, "error", err)
		}
	}

	if metadataStr != "" || mailboxesStr != "" {
		SetUserData(publicKey, metadataStr, mailboxesStr)
		log.ClientCache().Debug("Cached user data from objects", "pubkey", publicKey)
	}
}

// GetParsedUserData retrieves and parses cached user data
func GetParsedUserData(publicKey string) (*nostr.Event, *core.Mailboxes, bool) {
	cachedData, exists := GetUserData(publicKey)
	if !exists {
		return nil, nil, false
	}

	var metadata *nostr.Event
	var mailboxes *core.Mailboxes

	// Parse cached metadata
	if cachedData.Metadata != "" {
		var metadataEvent nostr.Event
		if err := json.Unmarshal([]byte(cachedData.Metadata), &metadataEvent); err != nil {
			log.ClientCache().Warn("Failed to parse cached metadata", "pubkey", publicKey, "error", err)
		} else {
			metadata = &metadataEvent
		}
	}

	// Parse cached mailboxes
	if cachedData.Mailboxes != "" && cachedData.Mailboxes != "{}" {
		var mailboxesData core.Mailboxes
		if err := json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxesData); err != nil {
			log.ClientCache().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
		} else {
			mailboxes = &mailboxesData
		}
	}

	return metadata, mailboxes, true
}

// IsValidCachedData checks if cached data contains valid user information
func IsValidCachedData(cachedData CachedUserData) bool {
	if cachedData.Metadata == "" {
		return false
	}

	// Try to parse metadata to ensure it's valid JSON
	var metadata nostr.Event
	if err := json.Unmarshal([]byte(cachedData.Metadata), &metadata); err != nil {
		return false
	}

	// Basic validation - must have ID and PubKey
	return metadata.ID != "" && metadata.PubKey != ""
}

// GetCacheStats returns statistics about the cache
func GetCacheStats() map[string]interface{} {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	totalEntries := len(cache.data)
	entriesWithMetadata := 0
	entriesWithMailboxes := 0

	for _, data := range cache.data {
		if data.Metadata != "" {
			entriesWithMetadata++
		}
		if data.Mailboxes != "" && data.Mailboxes != "{}" {
			entriesWithMailboxes++
		}
	}

	return map[string]interface{}{
		"total_entries":          totalEntries,
		"entries_with_metadata":  entriesWithMetadata,
		"entries_with_mailboxes": entriesWithMailboxes,
		"cache_expiry_minutes":   int(cache.expiry.Minutes()),
	}
}
