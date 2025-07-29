package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// AddClientRelayWithPermissions adds a client relay with specific permissions
func AddClientRelayWithPermissions(publicKey, relayURL string, read, write bool) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.clientRelays == nil {
		cache.clientRelays = make(map[string][]string)
	}

	// Create relay info with permissions
	relayInfo := ClientRelayConfig{
		URL:       relayURL,
		Read:      read,
		Write:     write,
		Connected: true, // Assume connected when adding
		AddedAt:   time.Now(),
	}

	// Serialize relay info
	relayData, err := json.Marshal(relayInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal relay info: %w", err)
	}

	// Store as JSON string in the clientRelays map
	cache.clientRelays[publicKey] = append(cache.clientRelays[publicKey], string(relayData))

	log.ClientCache().Info("Added client relay with permissions",
		"pubkey", publicKey,
		"relay", relayURL,
		"read", read,
		"write", write,
		"total_relays", len(cache.clientRelays[publicKey]))

	return nil
}

// GetUserClientRelaysWithPermissions retrieves user's client relays with permissions
func GetUserClientRelaysWithPermissions(publicKey string) ([]ClientRelayConfig, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	relayStrings, exists := cache.clientRelays[publicKey]
	if !exists || len(relayStrings) == 0 {
		return nil, nil
	}

	var relayConfigs []ClientRelayConfig

	for _, relayString := range relayStrings {
		var relayInfo ClientRelayConfig
		if err := json.Unmarshal([]byte(relayString), &relayInfo); err != nil {
			// Handle legacy format (plain URL strings)
			relayConfigs = append(relayConfigs, ClientRelayConfig{
				URL:       relayString,
				Read:      true,
				Write:     true,
				Connected: false, // Unknown status for legacy
				AddedAt:   time.Now(),
			})
			continue
		}

		relayConfigs = append(relayConfigs, relayInfo)
	}

	log.ClientCache().Debug("Retrieved user client relays with permissions",
		"pubkey", publicKey,
		"relay_count", len(relayConfigs))

	return relayConfigs, nil
}

// GetUserClientRelays returns user's client relays in the format expected by the API
func GetUserClientRelays(publicKey string) ([]ClientRelayConfig, error) {
	return GetUserClientRelaysWithPermissions(publicKey)
}

// ClearClientRelays clears all client relays for a user
func ClearClientRelays(publicKey string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.clientRelays != nil {
		delete(cache.clientRelays, publicKey)
		log.ClientCache().Debug("Cleared client relays", "pubkey", publicKey)
	}
}

// SetUserClientRelaysFromMailboxes sets user's client relays from cached mailbox data
func SetUserClientRelaysFromMailboxes(publicKey string) error {
	// Get cached user data
	cachedData, found := GetUserData(publicKey)
	if !found || cachedData.Mailboxes == "" {
		return fmt.Errorf("no cached mailboxes found for user %s", publicKey)
	}

	// Parse the cached mailboxes JSON
	var mailboxes struct {
		Read  []string `json:"read"`
		Write []string `json:"write"`
		Both  []string `json:"both"`
	}

	if err := json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxes); err != nil {
		return fmt.Errorf("failed to parse cached mailboxes: %w", err)
	}

	// Clear existing client relays
	ClearClientRelays(publicKey)

	// Add read relays
	for _, relayURL := range mailboxes.Read {
		if err := AddClientRelayWithPermissions(publicKey, relayURL, true, false); err != nil {
			log.ClientCache().Warn("Failed to add read relay",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	// Add write relays
	for _, relayURL := range mailboxes.Write {
		if err := AddClientRelayWithPermissions(publicKey, relayURL, false, true); err != nil {
			log.ClientCache().Warn("Failed to add write relay",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	// Add both relays (read and write)
	for _, relayURL := range mailboxes.Both {
		if err := AddClientRelayWithPermissions(publicKey, relayURL, true, true); err != nil {
			log.ClientCache().Warn("Failed to add both relay",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	log.ClientCache().Info("Set user client relays from cached mailboxes",
		"pubkey", publicKey,
		"read_count", len(mailboxes.Read),
		"write_count", len(mailboxes.Write),
		"both_count", len(mailboxes.Both))

	return nil
}

// RemoveClientRelay removes a specific relay from user's client relay list
func RemoveClientRelay(publicKey, relayURL string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.clientRelays == nil {
		return fmt.Errorf("no client relays found for user")
	}

	relayStrings, exists := cache.clientRelays[publicKey]
	if !exists || len(relayStrings) == 0 {
		return fmt.Errorf("no client relays found for user %s", publicKey)
	}

	// Find and remove the relay
	var updatedRelays []string
	var removed bool

	for _, relayString := range relayStrings {
		var relayInfo ClientRelayConfig
		if err := json.Unmarshal([]byte(relayString), &relayInfo); err != nil {
			// Handle legacy format (plain URL strings)
			if relayString != relayURL {
				updatedRelays = append(updatedRelays, relayString)
			} else {
				removed = true
			}
		} else {
			// Modern format with permissions
			if relayInfo.URL != relayURL {
				updatedRelays = append(updatedRelays, relayString)
			} else {
				removed = true
			}
		}
	}

	if !removed {
		return fmt.Errorf("relay %s not found in user's relay list", relayURL)
	}

	// Update the cache
	if len(updatedRelays) == 0 {
		delete(cache.clientRelays, publicKey)
	} else {
		cache.clientRelays[publicKey] = updatedRelays
	}

	log.ClientCache().Info("Removed client relay",
		"pubkey", publicKey,
		"relay", relayURL,
		"remaining_relays", len(updatedRelays))

	return nil
}
