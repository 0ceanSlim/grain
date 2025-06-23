package data

import (
	"fmt"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/server/utils/log"
)

// FetchAndCacheUserDataWithCoreClient fetches user data using the core client
func FetchAndCacheUserDataWithCoreClient(publicKey string) error {
	log.Util().Debug("Fetching fresh user data with core client", "pubkey", publicKey)

	// Ensure we have connected relays before proceeding
	if err := connection.EnsureRelayConnections(); err != nil {
		return fmt.Errorf("failed to ensure relay connections: %w", err)
	}

	// Get the core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		return fmt.Errorf("core client not available")
	}

	// First, try to get user's mailboxes using connected relays
	var relaysForMetadata []string
	mailboxes, err := coreClient.GetUserRelays(publicKey)
	if err != nil {
		log.Util().Warn("Failed to fetch mailboxes, using app relays", "pubkey", publicKey, "error", err)
		relaysForMetadata = connection.GetAppRelays()
	} else if mailboxes != nil {
		// Get user's preferred relays
		userRelays := mailboxes.ToStringSlice()
		log.Util().Debug("User has preferred relays", "pubkey", publicKey, "relay_count", len(userRelays))
		
		// BUT: Use connected app relays for profile fetch to ensure success
		// This is more reliable than trying to connect to user's personal relays
		connectedRelays := coreClient.GetConnectedRelays()
		if len(connectedRelays) > 0 {
			relaysForMetadata = connectedRelays
			log.Util().Debug("Using connected app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
		} else {
			relaysForMetadata = connection.GetAppRelays()
		}
	}

	// Use app relays as fallback
	if len(relaysForMetadata) == 0 {
		relaysForMetadata = connection.GetAppRelays()
		log.Util().Info("Using app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
	}

	// Fetch user metadata (profile) using connected relays
	userMetadata, err := coreClient.GetUserProfile(publicKey, relaysForMetadata)
	if err != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", err)
	}

	// Cache the data using the cache package function
	cache.CacheUserDataFromObjects(publicKey, userMetadata, mailboxes)

	log.Util().Info("User data fetched and cached successfully", "pubkey", publicKey)
	return nil
}