package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Client represents the main Nostr client with connection pooling
type Client struct {
	relayPool     *RelayPool
	subscriptions map[string]*Subscription
	config        *Config
	mu            sync.RWMutex

	// sessionRelays are the urls Acquired for the currently logged-in session
	// (a lease held on each for the session's lifetime). Guarded by mu. They're
	// released when the session switches relays or ends, so they can be
	// idle-evicted — the outbox pool stays additive rather than torn down.
	sessionRelays []string

	// directory resolves and caches per-target users' event-derived relay roles
	// (outbox / inbox / DM inbox) for outbox routing.
	directory *RelayDirectory

	// mediaDir resolves and caches per-user media-server lists (Blossom kind
	// 10063 / NIP-96 kind 10096) for the upload flow.
	mediaDir *MediaDirectory

	// relayListsCache caches a user's resolved relay lists (all kinds) so the
	// settings page renders from cache once login-hydration has warmed it.
	// relayListsInflight collapses concurrent resolves for the same pubkey onto
	// one network fetch (e.g. the relay-pool dropdown's warm prefetch and the
	// relay manager's own load), so the second caller waits for the first
	// instead of duplicating a multi-second cold resolve.
	relayListsMu       sync.Mutex
	relayListsCache    map[string]relayListsEntry
	relayListsInflight map[string]chan struct{}

	// Fixed-relay override (opt-out, default OFF). When enabled, every read uses
	// fixedRead and every write uses fixedWrite, bypassing outbox routing
	// entirely — replies stop reaching other users' inboxes. Only for users who
	// explicitly want a fixed-/single-relay client. Guarded by mu.
	fixedMode  bool
	fixedRead  []string
	fixedWrite []string

	// appRelays holds the session's editable overrides for the locally-configured
	// roles (Indexer/Broadcast/Local/Trusted), seeded from config. An unset role
	// falls back to config (the index relays) or has no effect. Guarded by
	// appRelaysMu. These are session preferences, not published Nostr lists.
	appRelaysMu sync.Mutex
	appRelays   map[Role][]string

	// relayInfoCache TTL-caches relays' NIP-11 documents for the known-relays
	// browser (#98). A plain HTTP GET per relay, not a pool connection.
	relayInfoMu    sync.Mutex
	relayInfoCache map[string]relayInfoEntry

	// relayPingCache short-TTL-caches TCP-connect latency per relay for the
	// known-relays "fastest first" sort (#98). A cheap reachability probe, not a
	// pool/WebSocket round-trip.
	relayPingMu    sync.Mutex
	relayPingCache map[string]relayPingEntry
}

// NewClient creates a new Nostr client instance
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	config.IndexRelays = normalizeRelayURLs(config.IndexRelays)

	c := &Client{
		relayPool:     NewRelayPool(config),
		subscriptions: make(map[string]*Subscription),
		config:        config,
	}
	c.directory = newRelayDirectory(config.RelayListTTL, config.RelayListNegTTL, c.fetchUserRelaysFromNetwork)
	c.mediaDir = newMediaDirectory(config.RelayListTTL, config.RelayListNegTTL, c.fetchUserMediaServersFromNetwork)
	c.relayListsCache = make(map[string]relayListsEntry)
	c.relayListsInflight = make(map[string]chan struct{})
	c.appRelays = make(map[Role][]string)
	c.relayInfoCache = make(map[string]relayInfoEntry)
	c.relayPingCache = make(map[string]relayPingEntry)
	return c
}

// ConnectToRelays establishes connections to multiple relay URLs
func (c *Client) ConnectToRelays(urls []string) error {
	log.ClientCore().Info("Connecting to relays", "relay_count", len(urls), "relays", urls)

	if len(urls) == 0 {
		return fmt.Errorf("no relay URLs provided")
	}

	var lastErr error
	connected := 0
	failed := []string{}

	for _, url := range urls {
		// Validate URL format
		if url == "" || (!strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://")) {
			log.ClientCore().Warn("Invalid relay URL format", "relay", url)
			failed = append(failed, url)
			lastErr = fmt.Errorf("invalid URL format: %s", url)
			continue
		}

		if err := c.relayPool.Connect(url); err != nil {
			log.ClientCore().Warn("Failed to connect to relay", "relay", url, "error", err)
			failed = append(failed, url)
			lastErr = err
			continue
		}
		connected++
		log.ClientCore().Debug("Successfully connected to relay", "relay", url)
	}

	if connected == 0 && lastErr != nil {
		log.ClientCore().Error("Failed to connect to any relays",
			"attempted", len(urls),
			"failed_relays", failed,
			"last_error", lastErr)
		return fmt.Errorf("failed to connect to any relays: %w", lastErr)
	}

	log.ClientCore().Info("Connected to relays",
		"connected", connected,
		"failed", len(failed),
		"total", len(urls))

	// Wait a moment for connections to stabilize
	time.Sleep(500 * time.Millisecond)

	// Verify connections are actually established
	actuallyConnected := c.GetConnectedRelays()
	log.ClientCore().Info("Relay connection verification",
		"reported_connected", connected,
		"actually_connected", len(actuallyConnected),
		"connected_relays", actuallyConnected)

	return nil
}

// DisconnectFromRelay closes a specific relay connection
func (c *Client) DisconnectFromRelay(relayURL string) error {
	log.ClientCore().Info("Disconnecting from relay", "relay", relayURL)

	// Use the relay pool's existing CloseConnection method
	if err := c.relayPool.CloseConnection(relayURL); err != nil {
		log.ClientCore().Error("Failed to close relay connection", "relay", relayURL, "error", err)
		return err
	}

	log.ClientCore().Info("Successfully disconnected from relay", "relay", relayURL)
	return nil
}

// DisconnectFromRelays closes connections to multiple relays
func (c *Client) DisconnectFromRelays(relayURLs []string) error {
	var lastErr error
	disconnected := 0

	log.ClientCore().Info("Disconnecting from multiple relays", "relay_count", len(relayURLs))

	for _, relayURL := range relayURLs {
		if err := c.DisconnectFromRelay(relayURL); err != nil {
			log.ClientCore().Warn("Failed to disconnect from relay", "relay", relayURL, "error", err)
			lastErr = err
		} else {
			disconnected++
		}
	}

	log.ClientCore().Info("Relay disconnection complete", "requested", len(relayURLs), "disconnected", disconnected)

	if disconnected == 0 && lastErr != nil {
		return fmt.Errorf("failed to disconnect from any relays: %w", lastErr)
	}

	return nil // Success if at least one disconnected
}

// Subscribe creates a new subscription with filters and relay hints
func (c *Client) Subscribe(ctx context.Context, filters []nostr.Filter, relayHints []string) (*Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err // already cancelled / past deadline
	}
	subID := generateSubscriptionID()

	// Use all connected relays if no hints provided
	targetRelays := relayHints
	if len(targetRelays) == 0 {
		targetRelays = c.relayPool.GetConnectedRelays()
	}

	if len(targetRelays) == 0 {
		return nil, &ClientError{Message: "no relays available for subscription"}
	}

	sub := NewSubscription(subID, filters, targetRelays, c)

	c.mu.Lock()
	c.subscriptions[subID] = sub
	c.mu.Unlock()

	log.ClientCore().Debug("Created subscription", "sub_id", subID, "relay_count", len(targetRelays))

	if err := sub.Start(); err != nil {
		c.mu.Lock()
		delete(c.subscriptions, subID)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to start subscription: %w", err)
	}

	return sub, nil
}

// GetConnectedRelays returns a list of currently connected relay URLs
func (c *Client) GetConnectedRelays() []string {
	return c.relayPool.GetConnectedRelays()
}

// GetRelayStatus returns detailed status of all relay connections
func (c *Client) GetRelayStatus() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make(map[string]string)
	connectedRelays := c.relayPool.GetConnectedRelays()

	// Mark connected relays
	for _, relay := range connectedRelays {
		status[relay] = "connected"
	}

	// Add configured relays that aren't connected
	for _, relay := range c.config.IndexRelays {
		if _, exists := status[relay]; !exists {
			status[relay] = "disconnected"
		}
	}

	return status
}

// ConnectToRelaysWithRetry establishes connections with retry logic
func (c *Client) ConnectToRelaysWithRetry(urls []string, maxRetries int) error {
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.ClientCore().Debug("Connection attempt", "attempt", attempt, "max_retries", maxRetries)

		err := c.ConnectToRelays(urls)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if any relays are connected
		connected := c.relayPool.GetConnectedRelays()
		if len(connected) > 0 {
			log.ClientCore().Info("Partial connection success", "connected_relays", len(connected))
			return nil // Partial success is acceptable
		}

		if attempt < maxRetries {
			delay := time.Duration(attempt) * c.config.RetryDelay
			log.ClientCore().Info("Retrying connection", "attempt", attempt, "delay", delay)
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, lastErr)
}

// collectLatestReplaceable drains a subscription and returns the newest event
// (by created_at) as soon as any relay signals EOSE, once every target relay
// has, or on timeout — whichever comes first. It returns nil when no matching
// event arrived; callers decide what that means (a hard error for a required
// profile, an empty result for an optional relay list).
//
// This is the shared core of GetUserProfile and GetUserRelays, and the fix for
// the GetUserRelays latency bug (#77): that method used to wait on sub.Done —
// which EOSE never closes — so every mailbox fetch burned the full timeout
// instead of returning the moment a relay reported EOSE.
func collectLatestReplaceable(ctx context.Context, sub *Subscription, totalRelays int, timeout time.Duration) *nostr.Event {
	deadline := time.After(timeout)
	eoseRelays := make(map[string]bool)
	var latest *nostr.Event

	for {
		select {
		case event := <-sub.Events:
			if event == nil {
				continue
			}
			// Keep the newest by created_at (these are replaceable events).
			if latest == nil || event.CreatedAt > latest.CreatedAt {
				latest = event
			}

		case relayURL := <-sub.EOSE:
			eoseRelays[relayURL] = true
			// Newest event in hand and a relay has nothing more to send.
			if latest != nil {
				return latest
			}
			// Every target relay reported end-of-stored-events; none had it.
			if len(eoseRelays) >= totalRelays {
				return nil
			}

		case err := <-sub.Errors:
			// One relay failing isn't fatal — others may still answer.
			log.ClientCore().Debug("Subscription error while collecting event",
				"sub_id", sub.ID, "error", err)

		case <-ctx.Done():
			return latest
		case <-deadline:
			return latest
		}
	}
}

func (c *Client) GetUserProfile(ctx context.Context, pubkey string, relayHints []string) (*nostr.Event, error) {
	log.ClientCore().Debug("Fetching user profile", "pubkey", pubkey, "relay_hints", relayHints)

	// Background-warm this user's mailbox relays into the directory (and thus
	// the "known" set), without blocking this fetch. Resolution only queries the
	// already-connected index relays, so it adds no new dials.
	c.WarmRelays(pubkey)

	// Create filter for metadata (kind 0)
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{0},
		Limit:   &[]int{1}[0], // Get latest only
	}

	// Use relay hints if provided, otherwise query the index/profile-indexer
	// relays plus the author's outbox when already cached (RouteMetadata) —
	// metadata lives on the indexers, and resolving the outbox synchronously on
	// every profile view is what made the dashboard crawl.
	targetRelays := relayHints
	if len(targetRelays) == 0 {
		targetRelays = c.RouteMetadata(pubkey)
		log.ClientCore().Debug("No relay hints provided, routing metadata fetch",
			"pubkey", pubkey, "relays", targetRelays)
	}

	sub, err := c.Subscribe(ctx, []nostr.Filter{filter}, targetRelays)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}
	defer sub.Close()

	event := collectLatestReplaceable(ctx, sub, len(targetRelays), 5*time.Second)
	if event == nil {
		log.ClientCore().Warn("No profile found", "pubkey", pubkey)
		return nil, &ClientError{Message: "profile not found"}
	}

	log.ClientCore().Debug("Fetched user profile",
		"pubkey", pubkey, "event_id", event.ID, "created_at", event.CreatedAt)
	return event, nil
}

// GetUserRelays retrieves a user's relay list (NIP-65), resolved through the
// directory — an index-relay query that is TTL-cached and single-flighted —
// rather than fanning a REQ out to every connected relay. That fan-out is what
// melted the dashboard during a mutelist sync once the outbox pool had grown to
// dozens of connections: each author's lookup blasted all of them. Returns
// empty mailboxes (not an error) when the user has published no relay list.
func (c *Client) GetUserRelays(pubkey string) (*Mailboxes, error) {
	ur := c.ResolveRelays(pubkey)

	// Reconstruct the read/write/both split callers expect from the directory's
	// outbox (write+both) / inbox (read+both) view: a relay in both is "both",
	// outbox-only is "write", inbox-only is "read".
	inbox := make(map[string]bool, len(ur.Inbox))
	for _, r := range ur.Inbox {
		inbox[r] = true
	}
	outbox := make(map[string]bool, len(ur.Outbox))
	for _, r := range ur.Outbox {
		outbox[r] = true
	}

	mb := &Mailboxes{}
	for _, r := range ur.Outbox {
		if inbox[r] {
			mb.Both = append(mb.Both, r)
		} else {
			mb.Write = append(mb.Write, r)
		}
	}
	for _, r := range ur.Inbox {
		if !outbox[r] {
			mb.Read = append(mb.Read, r)
		}
	}
	return mb, nil
}

// PublishEvent publishes an event to specified relays
func (c *Client) PublishEvent(ctx context.Context, event *nostr.Event, targetRelays []string) ([]BroadcastResult, error) {
	if event == nil {
		return nil, &ClientError{Message: "event cannot be nil"}
	}

	// Use connected relays if no target relays specified
	relays := targetRelays
	if len(relays) == 0 {
		relays = c.relayPool.GetConnectedRelays()
	}

	if len(relays) == 0 {
		return nil, &ClientError{Message: "no relays available for publishing"}
	}

	log.ClientCore().Info("Publishing event", "event_id", event.ID, "relay_count", len(relays))

	return BroadcastEvent(ctx, event, relays, c.relayPool), nil
}

// PublishEventWithRetry publishes an event with retry logic
func (c *Client) PublishEventWithRetry(ctx context.Context, event *nostr.Event, targetRelays []string, maxRetries int) ([]BroadcastResult, error) {
	if event == nil {
		return nil, &ClientError{Message: "event cannot be nil"}
	}

	// Use connected relays if no target relays specified
	relays := targetRelays
	if len(relays) == 0 {
		relays = c.relayPool.GetConnectedRelays()
	}

	if len(relays) == 0 {
		return nil, &ClientError{Message: "no relays available for publishing"}
	}

	log.ClientCore().Info("Publishing event with retry", "event_id", event.ID, "relay_count", len(relays), "max_retries", maxRetries)

	return BroadcastWithRetry(ctx, event, relays, c.relayPool, maxRetries), nil
}
func (c *Client) Close() error {
	log.ClientCore().Info("Shutting down client")

	// Close all subscriptions
	c.mu.Lock()
	for _, sub := range c.subscriptions {
		sub.Close()
	}
	c.subscriptions = make(map[string]*Subscription)
	c.mu.Unlock()

	// Close relay pool
	return c.relayPool.Close()
}

// ClientError represents client-specific errors
type ClientError struct {
	Message string
}

func (e *ClientError) Error() string {
	return e.Message
}

// subSeq guarantees subscription IDs are unique even when two subscriptions are
// created within the same microsecond — which happens now that login fetches
// mailboxes and metadata concurrently (#77). A collision would cross-wire the
// two subscriptions' events in the message router.
var subSeq atomic.Uint64

// generateSubscriptionID creates a unique subscription identifier
func generateSubscriptionID() string {
	return fmt.Sprintf("sub_%s_%d", time.Now().Format("20060102150405.000000"), subSeq.Add(1))
}

// parseMailboxEvent parses a kind 10002 event into a Mailboxes struct
func parseMailboxEvent(event *nostr.Event) *Mailboxes {
	if event.Kind != 10002 {
		log.ClientCore().Warn("Event is not a mailbox event", "kind", event.Kind, "expected", 10002)
		return &Mailboxes{}
	}

	mailboxes := &Mailboxes{}

	// Parse relay tags
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			relayURL, ok := normalizeRelayURL(tag[1])
			if !ok {
				continue
			}
			if len(tag) >= 3 {
				switch tag[2] {
				case "read":
					mailboxes.Read = append(mailboxes.Read, relayURL)
				case "write":
					mailboxes.Write = append(mailboxes.Write, relayURL)
				}
			} else {
				// No specific type means both read and write
				mailboxes.Both = append(mailboxes.Both, relayURL)
			}
		}
	}

	log.ClientCore().Debug("Parsed mailbox event", "event_id", event.ID,
		"read_count", len(mailboxes.Read),
		"write_count", len(mailboxes.Write),
		"both_count", len(mailboxes.Both))

	return mailboxes
}

// RelayConfig represents relay configuration with permissions
type RelayConfig struct {
	URL   string `json:"url"`
	Read  bool   `json:"read"`
	Write bool   `json:"write"`
}

// ReplaceRelayConnections swaps the relays held for the current session.
//
// In the outbox model this is ADDITIVE: it does not tear the shared pool down.
// It releases the previous session's leases (so those connections become
// idle-evictable once nothing else needs them) and acquires the new set,
// holding one lease on each for the session's lifetime. Index/seed relays are
// pinned separately and are never affected by a session switch.
func (c *Client) ReplaceRelayConnections(newRelays []RelayConfig) error {
	urls := make([]string, 0, len(newRelays))
	for _, relay := range newRelays {
		urls = append(urls, relay.URL)
	}

	// Drop the previous session's holds first.
	c.mu.Lock()
	previous := c.sessionRelays
	c.sessionRelays = nil
	c.mu.Unlock()
	for _, u := range previous {
		c.relayPool.Release(u)
	}

	// Acquire the new set, holding one lease each for the session duration.
	acquired := make([]string, 0, len(urls))
	for _, u := range urls {
		if _, err := c.relayPool.Acquire(u); err != nil {
			log.ClientCore().Debug("Failed to acquire session relay", "relay", u, "error", err)
			continue
		}
		acquired = append(acquired, u)
	}

	c.mu.Lock()
	c.sessionRelays = acquired
	c.mu.Unlock()

	if len(acquired) == 0 && len(urls) > 0 {
		return fmt.Errorf("failed to connect to any of the %d requested relays", len(urls))
	}

	log.ClientCore().Info("Switched session relays (additive)",
		"requested", len(urls), "acquired", len(acquired))
	return nil
}

// SwitchToUserRelays holds the user's relays for the session (additive — see
// ReplaceRelayConnections). Index relays stay connected alongside them.
func (c *Client) SwitchToUserRelays(userRelays []RelayConfig) error {
	if len(userRelays) == 0 {
		log.ClientCore().Warn("No user relays provided, keeping current session relays")
		return nil
	}
	log.ClientCore().Info("Switching to user relays", "relay_count", len(userRelays))
	return c.ReplaceRelayConnections(userRelays)
}

// SwitchToIndexRelays releases the current session's relay leases. There is
// nothing to "switch back" to in the additive model: the index/seed relays are
// pinned and kept up by the health check, so they remain connected regardless.
func (c *Client) SwitchToIndexRelays() error {
	c.mu.Lock()
	previous := c.sessionRelays
	c.sessionRelays = nil
	c.mu.Unlock()
	for _, u := range previous {
		c.relayPool.Release(u)
	}
	log.ClientCore().Info("Released session relays; pinned index relays remain", "released", len(previous))
	return nil
}

// PinRelays marks relays so the idle sweeper never evicts them — used for the
// index/seed relays. Pinning does not dial; the connection is still established
// on demand by Acquire or the startup connect.
func (c *Client) PinRelays(urls ...string) {
	c.relayPool.Pin(urls...)
}

// StartEvictionSweeper starts the pool's idle-connection sweeper, bounded to ctx.
func (c *Client) StartEvictionSweeper(ctx context.Context, interval time.Duration) {
	c.relayPool.StartEvictionSweeper(ctx, interval)
}
