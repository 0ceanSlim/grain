package data

import (
	"context"
	"fmt"
	"sync"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Single-flight dedup for user-data fetches. Concurrent callers for the same
// pubkey (e.g. session creation + a client polling /api/v1/cache while the
// profile hydrates) collapse onto one outbox fetch instead of stampeding cold
// relays.
var (
	flightMu sync.Mutex
	flights  = map[string]chan struct{}{}
)

// FetchUserDataDeduped fetches + caches the user's data, collapsing concurrent
// calls for the same pubkey into a single fetch. It BLOCKS until the in-flight
// fetch finishes, so call it from a goroutine when you don't want to wait.
func FetchUserDataDeduped(publicKey string) {
	flightMu.Lock()
	if ch, ok := flights[publicKey]; ok {
		flightMu.Unlock()
		<-ch // a fetch is already running — wait for it
		return
	}
	ch := make(chan struct{})
	flights[publicKey] = ch
	flightMu.Unlock()

	if err := FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
		log.ClientData().Warn("User-data fetch failed", "pubkey", publicKey, "error", err)
	}

	flightMu.Lock()
	delete(flights, publicKey)
	flightMu.Unlock()
	close(ch)
}

// EnsureBackgroundFetch kicks a deduplicated user-data fetch and returns
// immediately — login and the cache endpoint use it so neither blocks on cold
// outbox relays. Safe to call repeatedly.
func EnsureBackgroundFetch(publicKey string) {
	go FetchUserDataDeduped(publicKey)
}

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
	defaultRelays := connection.GetIndexRelays()
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

	// Fetch mailboxes (kind 10002) and metadata (kind 0) concurrently. They are
	// independent queries, and running them serially was a dominant chunk of
	// login latency (#77): each waits on its own relay round-trip, so back to
	// back they doubled the worst case. Subscription IDs are unique per call
	// (see generateSubscriptionID), so the two REQs don't cross-wire.
	var (
		mailboxes    *core.Mailboxes
		userMetadata *nostr.Event
		mailboxErr   error
		profileErr   error
		wg           sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		log.ClientData().Info("Fetching user mailboxes", "pubkey", publicKey, "relay_count", len(relaysForQueries))
		mailboxes, mailboxErr = coreClient.GetUserRelays(publicKey)
	}()
	go func() {
		defer wg.Done()
		log.ClientData().Debug("Fetching user metadata", "pubkey", publicKey)
		userMetadata, profileErr = coreClient.GetUserProfile(context.Background(), publicKey, relaysForQueries)
	}()
	wg.Wait()

	// Mailboxes are optional — the user may not have published a relay list.
	if mailboxErr != nil {
		log.ClientData().Warn("Failed to fetch mailboxes, user may not have relay list published",
			"pubkey", publicKey,
			"error", mailboxErr,
			"relays_used", relaysForQueries)
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

	// Metadata is required.
	if profileErr != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", profileErr)
	}
	log.ClientData().Info("Successfully fetched user metadata", "pubkey", publicKey, "event_id", userMetadata.ID)

	// Cache the data using the cache package function
	cache.CacheUserDataFromObjects(publicKey, userMetadata, mailboxes)

	// Login-hydration: warm the media-server + relay-list caches in the
	// background so the settings pages render from cache instead of fetching on
	// open. Both Warm* helpers spawn their own goroutine, so this never blocks.
	coreClient.WarmMediaServers(publicKey)
	coreClient.WarmUserRelayLists(publicKey)

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
