package data

import (
	"fmt"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/server/utils/log"
)

// FetchAndCacheUserDataWithCoreClient fetches user data using the core client
func FetchAndCacheUserDataWithCoreClient(publicKey string) error {
	log.ClientData().Debug("Fetching fresh user data with core client", "pubkey", publicKey)

	// Ensure we have connected relays before proceeding
	if err := connection.EnsureRelayConnections(); err != nil {
		return fmt.Errorf("failed to ensure relay connections: %w", err)
	}

	// Get the core client
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		return fmt.Errorf("core client not available")
	}

	// Get default client relays as fallback
	defaultRelays := connection.GetClientRelays()

	// First, try to get user's mailboxes using default relays
	log.ClientData().Debug("Fetching user mailboxes from default relays", "pubkey", publicKey, "relay_count", len(defaultRelays))
	mailboxes, err := coreClient.GetUserRelays(publicKey)
	if err != nil {
		log.ClientData().Warn("Failed to fetch mailboxes from default relays", "pubkey", publicKey, "error", err)
		mailboxes = nil
	}

	// Determine which relays to use for metadata fetch and connect if needed
	var relaysForMetadata []string
	if mailboxes != nil {
		// User has mailboxes - connect to them for metadata fetch
		userRelays := mailboxes.ToStringSlice()
		log.ClientData().Debug("User has mailboxes, ensuring connection to user relays", "pubkey", publicKey, "relay_count", len(userRelays))

		// Ensure we're connected to user's preferred relays
		if err := coreClient.ConnectToRelaysWithRetry(userRelays, 2); err != nil {
			log.ClientData().Warn("Failed to connect to user relays, using default relays", "pubkey", publicKey, "error", err)
			relaysForMetadata = defaultRelays
		} else {
			// Successfully connected, use user relays for metadata
			// Give preference to user relays over default relays by using only user relays
			log.ClientData().Info("Successfully connected to user relays", "pubkey", publicKey, "relay_count", len(userRelays))
			relaysForMetadata = userRelays
		}

		// Initialize user's client relays from their mailboxes (cache them)
		if err := initializeClientRelaysFromMailboxes(publicKey, mailboxes); err != nil {
			log.ClientData().Warn("Failed to initialize client relays from mailboxes", "pubkey", publicKey, "error", err)
		}
	} else {
		// No mailboxes found - use default relays for metadata
		log.ClientData().Debug("No user mailboxes found, using default relays for metadata", "pubkey", publicKey, "relay_count", len(defaultRelays))
		relaysForMetadata = defaultRelays

		// Initialize client relays with default relays
		if err := initializeClientRelaysFromDefaults(publicKey, defaultRelays); err != nil {
			log.ClientData().Warn("Failed to initialize client relays from defaults", "pubkey", publicKey, "error", err)
		}
	}

	// Fetch user metadata (profile) using the determined relays
	userMetadata, err := coreClient.GetUserProfile(publicKey, relaysForMetadata)
	if err != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", err)
	}

	// Cache the data using the cache package function
	cache.CacheUserDataFromObjects(publicKey, userMetadata, mailboxes)

	log.ClientData().Info("User data fetched and cached successfully", "pubkey", publicKey)
	return nil
}

// initializeClientRelaysFromMailboxes sets up user's client relays from their mailboxes
func initializeClientRelaysFromMailboxes(publicKey string, mailboxes *core.Mailboxes) error {
	if mailboxes == nil {
		return fmt.Errorf("mailboxes is nil")
	}

	userRelays := mailboxes.ToStringSlice()
	log.ClientData().Info("Initializing client relays from user mailboxes",
		"pubkey", publicKey,
		"relay_count", len(userRelays))

	// Clear any existing client relays and add the user's mailbox relays
	// This replaces default relays with user's preferred relays
	for _, relayURL := range userRelays {
		if err := cache.AddClientRelay(publicKey, relayURL); err != nil {
			log.ClientData().Warn("Failed to add client relay from mailbox",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	return nil
}

// initializeClientRelaysFromDefaults sets up user's client relays from default app relays
func initializeClientRelaysFromDefaults(publicKey string, defaultRelays []string) error {
	log.ClientData().Info("Initializing client relays from default relays",
		"pubkey", publicKey,
		"relay_count", len(defaultRelays))

	// Add default relays as user's initial client relays
	for _, relayURL := range defaultRelays {
		if err := cache.AddClientRelay(publicKey, relayURL); err != nil {
			log.ClientData().Warn("Failed to add default client relay",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	return nil
}
