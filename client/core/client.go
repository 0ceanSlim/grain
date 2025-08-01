package core

import (
	"fmt"
	"sync"
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
}

// NewClient creates a new Nostr client instance
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	return &Client{
		relayPool:     NewRelayPool(config),
		subscriptions: make(map[string]*Subscription),
		config:        config,
		mu:            sync.RWMutex{},
	}
}

// ConnectToRelays establishes connections to multiple relay URLs
func (c *Client) ConnectToRelays(urls []string) error {
	log.ClientCore().Info("Connecting to relays", "relay_count", len(urls))

	var lastErr error
	connected := 0

	for _, url := range urls {
		if err := c.relayPool.Connect(url); err != nil {
			log.ClientCore().Warn("Failed to connect to relay", "relay", url, "error", err)
			lastErr = err
			continue
		}
		connected++
	}

	if connected == 0 && lastErr != nil {
		return fmt.Errorf("failed to connect to any relays: %w", lastErr)
	}

	log.ClientCore().Info("Connected to relays", "connected", connected, "total", len(urls))

	// Wait a moment for connections to stabilize
	time.Sleep(500 * time.Millisecond)

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
func (c *Client) Subscribe(filters []nostr.Filter, relayHints []string) (*Subscription, error) {
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
	for _, relay := range c.config.DefaultRelays {
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
func (c *Client) GetUserProfile(pubkey string, relayHints []string) (*nostr.Event, error) {
	log.ClientCore().Debug("Fetching user profile", "pubkey", pubkey)

	// Create filter for metadata (kind 0)
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{0},
		Limit:   &[]int{1}[0], // Get latest only
	}

	// Subscribe with timeout
	sub, err := c.Subscribe([]nostr.Filter{filter}, relayHints)
	if err != nil {
		return nil, err
	}
	defer sub.Close()

	// Wait for events with timeout
	timeout := time.After(5 * time.Second)

	select {
	case event := <-sub.Events:
		log.ClientCore().Debug("Received user profile", "pubkey", pubkey, "event_id", event.ID)
		return event, nil
	case err := <-sub.Errors:
		return nil, err
	case <-timeout:
		return nil, &ClientError{Message: "timeout waiting for profile"}
	case <-sub.Done:
		return nil, &ClientError{Message: "subscription closed before profile received"}
	}
}

// GetUserRelays retrieves user relay list (kind 10002)
func (c *Client) GetUserRelays(pubkey string) (*Mailboxes, error) {
	log.ClientCore().Debug("Fetching user relays", "pubkey", pubkey)

	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{10002},
		Limit:   &[]int{1}[0],
	}

	// Use connected relays for relay list queries
	connectedRelays := c.relayPool.GetConnectedRelays()
	if len(connectedRelays) == 0 {
		return nil, &ClientError{Message: "no connected relays available"}
	}

	sub, err := c.Subscribe([]nostr.Filter{filter}, connectedRelays)
	if err != nil {
		return nil, err
	}
	defer sub.Close()

	timeout := time.After(5 * time.Second)

	select {
	case event := <-sub.Events:
		mailboxes := parseMailboxEvent(event)
		log.ClientCore().Debug("Received user relays", "pubkey", pubkey,
			"read_count", len(mailboxes.Read),
			"write_count", len(mailboxes.Write),
			"both_count", len(mailboxes.Both))
		return mailboxes, nil
	case err := <-sub.Errors:
		return nil, err
	case <-timeout:
		return nil, &ClientError{Message: "timeout waiting for relay list"}
	case <-sub.Done:
		return nil, &ClientError{Message: "subscription completed without receiving relay list"}
	}
}

// PublishEvent publishes an event to specified relays
func (c *Client) PublishEvent(event *nostr.Event, targetRelays []string) ([]BroadcastResult, error) {
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

	return BroadcastEvent(event, relays, c.relayPool), nil
}

// PublishEventWithRetry publishes an event with retry logic
func (c *Client) PublishEventWithRetry(event *nostr.Event, targetRelays []string, maxRetries int) ([]BroadcastResult, error) {
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

	return BroadcastWithRetry(event, relays, c.relayPool, maxRetries), nil
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

// generateSubscriptionID creates a unique subscription identifier
func generateSubscriptionID() string {
	// Simple time-based ID for now
	return "sub_" + time.Now().Format("20060102150405.000000")
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
			relayURL := tag[1]
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

// ReplaceRelayConnections replaces current relay connections with a new set
func (c *Client) ReplaceRelayConnections(newRelays []RelayConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.ClientCore().Info("Replacing relay connections", "new_relay_count", len(newRelays))

	// Close existing connections
	if err := c.relayPool.Close(); err != nil {
		log.ClientCore().Warn("Error closing existing relay pool", "error", err)
	}

	// Create new relay pool with current config
	c.relayPool = NewRelayPool(c.config)

	// Extract URLs for connection
	var relayURLs []string
	for _, relay := range newRelays {
		relayURLs = append(relayURLs, relay.URL)
	}

	// Connect to new relays
	if err := c.ConnectToRelaysWithRetry(relayURLs, 2); err != nil {
		return fmt.Errorf("failed to connect to new relay set: %w", err)
	}

	// Log relay permissions
	for _, relay := range newRelays {
		permissions := []string{}
		if relay.Read {
			permissions = append(permissions, "read")
		}
		if relay.Write {
			permissions = append(permissions, "write")
		}
		log.ClientCore().Debug("Relay permissions set",
			"relay", relay.URL,
			"permissions", permissions)
	}

	log.ClientCore().Info("Successfully replaced relay connections",
		"connected_count", len(c.GetConnectedRelays()))

	return nil
}

// SwitchToUserRelays switches the client to use user's cached relays
func (c *Client) SwitchToUserRelays(userRelays []RelayConfig) error {
	log.ClientCore().Info("Switching to user relays", "relay_count", len(userRelays))

	if len(userRelays) == 0 {
		log.ClientCore().Warn("No user relays found, keeping current connections")
		return nil
	}

	// Replace connections with user's relays
	return c.ReplaceRelayConnections(userRelays)
}

// SwitchToDefaultRelays switches the client back to default app relays
func (c *Client) SwitchToDefaultRelays() error {
	log.ClientCore().Info("Switching to default app relays")

	// Convert default relays to RelayConfig format (both read and write)
	var defaultRelayConfigs []RelayConfig
	for _, url := range c.config.DefaultRelays {
		defaultRelayConfigs = append(defaultRelayConfigs, RelayConfig{
			URL:   url,
			Read:  true,
			Write: true,
		})
	}

	// Replace connections with default relays
	return c.ReplaceRelayConnections(defaultRelayConfigs)
}
