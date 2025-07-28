package connection

import (
	"fmt"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/server/utils/log"
)

// SwitchToUserRelays switches the core client to use user's cached relays from mailboxes
func SwitchToUserRelays(publicKey string) error {
	coreClient := GetCoreClient()
	if coreClient == nil {
		return fmt.Errorf("core client not available")
	}

	// Get user's cached mailboxes from cache
	cachedData, found := cache.GetUserData(publicKey)
	if !found || cachedData.Mailboxes == "" {
		log.ClientConnection().Warn("No cached mailboxes found for user", "pubkey", publicKey)
		return nil
	}

	// Parse the cached mailboxes
	_, mailboxes, found := cache.GetParsedUserData(publicKey)
	if !found || mailboxes == nil {
		log.ClientConnection().Warn("Failed to parse cached mailboxes", "pubkey", publicKey)
		return nil
	}

	// Convert mailboxes to core.RelayConfig format
	var userRelayConfigs []core.RelayConfig

	// Add read relays
	for _, relayURL := range mailboxes.Read {
		userRelayConfigs = append(userRelayConfigs, core.RelayConfig{
			URL:   relayURL,
			Read:  true,
			Write: false,
		})
	}

	// Add write relays
	for _, relayURL := range mailboxes.Write {
		userRelayConfigs = append(userRelayConfigs, core.RelayConfig{
			URL:   relayURL,
			Read:  false,
			Write: true,
		})
	}

	// Add both relays (read and write)
	for _, relayURL := range mailboxes.Both {
		userRelayConfigs = append(userRelayConfigs, core.RelayConfig{
			URL:   relayURL,
			Read:  true,
			Write: true,
		})
	}

	if len(userRelayConfigs) == 0 {
		log.ClientConnection().Warn("No user relays found in mailboxes", "pubkey", publicKey)
		return nil
	}

	log.ClientConnection().Info("Switching to user relays from cached mailboxes",
		"pubkey", publicKey,
		"relay_count", len(userRelayConfigs))

	// Use the core client method that takes core.RelayConfig slice
	return coreClient.SwitchToUserRelays(userRelayConfigs)
}

// SwitchToDefaultRelays switches the core client back to default relays
func SwitchToDefaultRelays() error {
	coreClient := GetCoreClient()
	if coreClient == nil {
		return fmt.Errorf("core client not available")
	}

	return coreClient.SwitchToDefaultRelays()
}
