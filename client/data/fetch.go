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
	connectedRelays := coreClient.GetConnectedRelays()

	// Use connected relays if available, otherwise fall back to default
	var relaysForQueries []string
	if len(connectedRelays) > 0 {
		relaysForQueries = connectedRelays
		log.ClientData().Debug("Using connected relays for queries", "pubkey", publicKey, "relay_count", len(connectedRelays))
	} else {
		relaysForQueries = defaultRelays
		log.ClientData().Debug("Using default relays for queries", "pubkey", publicKey, "relay_count", len(defaultRelays))
	}

	if len(relaysForQueries) == 0 {
		return fmt.Errorf("no relays available for fetching user data")
	}

	// Fetch user's mailboxes (kind 10002) first with better error handling
	log.ClientData().Info("Fetching user mailboxes", "pubkey", publicKey, "relay_count", len(relaysForQueries))
	mailboxes, err := coreClient.GetUserRelays(publicKey)
	if err != nil {
		log.ClientData().Warn("Failed to fetch mailboxes, user may not have relay list published",
			"pubkey", publicKey,
			"error", err,
			"relays_used", relaysForQueries)
		// Set mailboxes to nil - this is not an error, user may not have published a relay list
		mailboxes = nil
	} else if mailboxes != nil {
		totalRelays := len(mailboxes.Read) + len(mailboxes.Write) + len(mailboxes.Both)
		log.ClientData().Info("Successfully fetched user mailboxes",
			"pubkey", publicKey,
			"read_count", len(mailboxes.Read),
			"write_count", len(mailboxes.Write),
			"both_count", len(mailboxes.Both),
			"total_relays", totalRelays)
	}

	// Fetch user metadata (profile) using the same relays
	log.ClientData().Debug("Fetching user metadata", "pubkey", publicKey)
	userMetadata, err := coreClient.GetUserProfile(publicKey, relaysForQueries)
	if err != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", err)
	}

	log.ClientData().Info("Successfully fetched user metadata", "pubkey", publicKey, "event_id", userMetadata.ID)

	// Cache the data using the cache package function
	cache.CacheUserDataFromObjects(publicKey, userMetadata, mailboxes)

	// Initialize client relays based on what we found
	if mailboxes != nil {
		// User has mailboxes - replace client relays with user's preferred relays
		if err := initializeClientRelaysFromMailboxes(publicKey, mailboxes); err != nil {
			log.ClientData().Warn("Failed to initialize client relays from mailboxes", "pubkey", publicKey, "error", err)
		}
	} else {
		// No mailboxes found - initialize with default relays
		if err := initializeClientRelaysFromDefaults(publicKey, defaultRelays); err != nil {
			log.ClientData().Warn("Failed to initialize client relays from defaults", "pubkey", publicKey, "error", err)
		}
	}

	log.ClientData().Info("User data fetched and cached successfully", "pubkey", publicKey)
	return nil
}

// initializeClientRelaysFromMailboxes sets up user's client relays from their mailboxes
func initializeClientRelaysFromMailboxes(publicKey string, mailboxes *core.Mailboxes) error {
	if mailboxes == nil {
		return fmt.Errorf("mailboxes is nil")
	}

	// CRITICAL FIX: Clear any existing client relays FIRST
	cache.ClearClientRelays(publicKey)

	userRelays := mailboxes.ToStringSlice()
	log.ClientData().Info("Replacing client relays with user mailboxes",
		"pubkey", publicKey,
		"relay_count", len(userRelays))

	// Add read relays with proper permissions
	for _, relayURL := range mailboxes.Read {
		if err := cache.AddClientRelayWithPermissions(publicKey, relayURL, true, false); err != nil {
			log.ClientData().Warn("Failed to add read relay from mailbox",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	// Add write relays with proper permissions
	for _, relayURL := range mailboxes.Write {
		if err := cache.AddClientRelayWithPermissions(publicKey, relayURL, false, true); err != nil {
			log.ClientData().Warn("Failed to add write relay from mailbox",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	// Add both relays (read and write permissions)
	for _, relayURL := range mailboxes.Both {
		if err := cache.AddClientRelayWithPermissions(publicKey, relayURL, true, true); err != nil {
			log.ClientData().Warn("Failed to add both relay from mailbox",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	log.ClientData().Info("Client relays replaced with user mailboxes",
		"pubkey", publicKey,
		"read_count", len(mailboxes.Read),
		"write_count", len(mailboxes.Write),
		"both_count", len(mailboxes.Both),
		"total_relays", len(userRelays))

	return nil
}

// initializeClientRelaysFromDefaults sets up user's client relays from default app relays
func initializeClientRelaysFromDefaults(publicKey string, defaultRelays []string) error {
	// CRITICAL FIX: Clear existing client relays first
	cache.ClearClientRelays(publicKey)

	log.ClientData().Info("Replacing client relays with default relays",
		"pubkey", publicKey,
		"relay_count", len(defaultRelays))

	// Add default relays as user's initial client relays (both read and write)
	for _, relayURL := range defaultRelays {
		if err := cache.AddClientRelayWithPermissions(publicKey, relayURL, true, true); err != nil {
			log.ClientData().Warn("Failed to add default client relay",
				"pubkey", publicKey,
				"relay", relayURL,
				"error", err)
		}
	}

	log.ClientData().Info("Client relays replaced with default relays",
		"pubkey", publicKey,
		"total_relays", len(defaultRelays))

	return nil
}
