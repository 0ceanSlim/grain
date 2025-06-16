package core

import (
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
	log.Util().Info("Connecting to relays", "relay_count", len(urls))
	
	var lastErr error
	connected := 0
	
	for _, url := range urls {
		if err := c.relayPool.Connect(url); err != nil {
			log.Util().Warn("Failed to connect to relay", "relay", url, "error", err)
			lastErr = err
			continue
		}
		connected++
	}
	
	if connected == 0 && lastErr != nil {
		return lastErr
	}
	
	log.Util().Info("Connected to relays", "connected", connected, "total", len(urls))
	return nil
}

// Subscribe creates a new subscription with filters and relay hints
func (c *Client) Subscribe(filters []nostr.Filter, relayHints []string) (*Subscription, error) {
	subID := generateSubscriptionID()
	
	// Use all connected relays if no hints provided
	targetRelays := relayHints
	if len(targetRelays) == 0 {
		targetRelays = c.relayPool.GetConnectedRelays()
	}
	
	sub := NewSubscription(subID, filters, targetRelays, c)
	
	c.mu.Lock()
	c.subscriptions[subID] = sub
	c.mu.Unlock()
	
	log.Util().Debug("Created subscription", "sub_id", subID, "relay_count", len(targetRelays))
	
	if err := sub.Start(); err != nil {
		c.mu.Lock()
		delete(c.subscriptions, subID)
		c.mu.Unlock()
		return nil, err
	}
	
	return sub, nil
}

// GetUserProfile retrieves user profile data using the core client
func (c *Client) GetUserProfile(pubkey string, relayHints []string) (*nostr.Event, error) {
	log.Util().Debug("Fetching user profile", "pubkey", pubkey)
	
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
		log.Util().Debug("Received user profile", "pubkey", pubkey, "event_id", event.ID)
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
	log.Util().Debug("Fetching user relays", "pubkey", pubkey)
	
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{10002},
		Limit:   &[]int{1}[0],
	}
	
	sub, err := c.Subscribe([]nostr.Filter{filter}, nil)
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	
	timeout := time.After(5 * time.Second)
	
	select {
	case event := <-sub.Events:
		mailboxes := parseMailboxEvent(event)
		log.Util().Debug("Received user relays", "pubkey", pubkey, 
			"read_count", len(mailboxes.Read),
			"write_count", len(mailboxes.Write),
			"both_count", len(mailboxes.Both))
		return mailboxes, nil
	case err := <-sub.Errors:
		return nil, err
	case <-timeout:
		return nil, &ClientError{Message: "timeout waiting for relay list"}
	case <-sub.Done:
		return nil, &ClientError{Message: "subscription closed before relay list received"}
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
	
	log.Util().Info("Publishing event", "event_id", event.ID, "relay_count", len(relays))
	
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
	
	log.Util().Info("Publishing event with retry", "event_id", event.ID, "relay_count", len(relays), "max_retries", maxRetries)
	
	return BroadcastWithRetry(event, relays, c.relayPool, maxRetries), nil
}
func (c *Client) Close() error {
	log.Util().Info("Shutting down client")
	
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
		log.Util().Warn("Event is not a mailbox event", "kind", event.Kind, "expected", 10002)
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
	
	log.Util().Debug("Parsed mailbox event", "event_id", event.ID,
		"read_count", len(mailboxes.Read),
		"write_count", len(mailboxes.Write),
		"both_count", len(mailboxes.Both))
	
	return mailboxes
}