package cache

import (
	"encoding/json"

	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

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
