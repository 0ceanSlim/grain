package core

import (
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Subscription manages a Nostr subscription across multiple relays
type Subscription struct {
	ID       string
	Filters  []nostr.Filter
	Relays   []string
	Events   chan *nostr.Event
	Errors   chan error
	Done     chan struct{}
	client   *Client
	mu       sync.RWMutex
	active   bool
}

// NewSubscription creates a new subscription instance
func NewSubscription(id string, filters []nostr.Filter, relays []string, client *Client) *Subscription {
	return &Subscription{
		ID:      id,
		Filters: filters,
		Relays:  relays,
		Events:  make(chan *nostr.Event, 100), // Buffered channel
		Errors:  make(chan error, 10),
		Done:    make(chan struct{}),
		client:  client,
		active:  false,
	}
}

// Start begins the subscription on all specified relays
func (s *Subscription) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.active {
		return &ClientError{Message: "subscription already active"}
	}
	
	log.Util().Debug("Starting subscription", "sub_id", s.ID, "relay_count", len(s.Relays))
	
	// Register with relay pool for message routing
	s.client.relayPool.RegisterSubscription(s.ID, s)
	
	// Send REQ message to all relays
	reqMessage := []interface{}{"REQ", s.ID}
	for _, filter := range s.Filters {
		reqMessage = append(reqMessage, filter)
	}
	
	var lastErr error
	sent := 0
	
	for _, relayURL := range s.Relays {
		if err := s.client.relayPool.SendMessage(relayURL, reqMessage); err != nil {
			log.Util().Warn("Failed to send subscription to relay", "relay", relayURL, "sub_id", s.ID, "error", err)
			lastErr = err
			continue
		}
		
		// Mark relay as having this subscription
		if conn, err := s.client.relayPool.GetConnection(relayURL); err == nil {
			conn.mu.Lock()
			conn.Subscriptions[s.ID] = true
			conn.mu.Unlock()
		}
		
		sent++
	}
	
	if sent == 0 && lastErr != nil {
		// Unregister since we failed to start
		s.client.relayPool.UnregisterSubscription(s.ID)
		return lastErr
	}
	
	s.active = true
	
	// Start message processor
	go s.processMessages()
	
	log.Util().Info("Subscription started", "sub_id", s.ID, "sent_to", sent, "total_relays", len(s.Relays))
	return nil
}

// Close terminates the subscription
func (s *Subscription) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.active {
		return nil
	}
	
	log.Util().Debug("Closing subscription", "sub_id", s.ID)
	
	// Unregister from relay pool
	s.client.relayPool.UnregisterSubscription(s.ID)
	
	// Send CLOSE message to all relays
	closeMessage := []interface{}{"CLOSE", s.ID}
	
	for _, relayURL := range s.Relays {
		if err := s.client.relayPool.SendMessage(relayURL, closeMessage); err != nil {
			log.Util().Warn("Failed to send close to relay", "relay", relayURL, "sub_id", s.ID, "error", err)
		}
		
		// Remove subscription from relay
		if conn, err := s.client.relayPool.GetConnection(relayURL); err == nil {
			conn.mu.Lock()
			delete(conn.Subscriptions, s.ID)
			conn.mu.Unlock()
		}
	}
	
	s.active = false
	close(s.Done)
	close(s.Events)
	close(s.Errors)
	
	log.Util().Debug("Subscription closed", "sub_id", s.ID)
	return nil
}

// AddRelay adds a new relay to an active subscription
func (s *Subscription) AddRelay(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check if relay is already in the list
	for _, existingURL := range s.Relays {
		if existingURL == url {
			return &ClientError{Message: "relay already in subscription"}
		}
	}
	
	// Add to relay list
	s.Relays = append(s.Relays, url)
	
	// If subscription is active, send REQ to new relay
	if s.active {
		reqMessage := []interface{}{"REQ", s.ID}
		for _, filter := range s.Filters {
			reqMessage = append(reqMessage, filter)
		}
		
		if err := s.client.relayPool.SendMessage(url, reqMessage); err != nil {
			// Remove from list if send failed
			s.Relays = s.Relays[:len(s.Relays)-1]
			return err
		}
		
		// Mark relay as having this subscription
		if conn, err := s.client.relayPool.GetConnection(url); err == nil {
			conn.mu.Lock()
			conn.Subscriptions[s.ID] = true
			conn.mu.Unlock()
		}
	}
	
	log.Util().Debug("Relay added to subscription", "sub_id", s.ID, "relay", url)
	return nil
}

// RemoveRelay removes a relay from the subscription
func (s *Subscription) RemoveRelay(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Find and remove relay from list
	found := false
	for i, existingURL := range s.Relays {
		if existingURL == url {
			s.Relays = append(s.Relays[:i], s.Relays[i+1:]...)
			found = true
			break
		}
	}
	
	if !found {
		return &ClientError{Message: "relay not found in subscription"}
	}
	
	// If subscription is active, send CLOSE to removed relay
	if s.active {
		closeMessage := []interface{}{"CLOSE", s.ID}
		if err := s.client.relayPool.SendMessage(url, closeMessage); err != nil {
			log.Util().Warn("Failed to send close to removed relay", "relay", url, "sub_id", s.ID, "error", err)
		}
		
		// Remove subscription from relay
		if conn, err := s.client.relayPool.GetConnection(url); err == nil {
			conn.mu.Lock()
			delete(conn.Subscriptions, s.ID)
			conn.mu.Unlock()
		}
	}
	
	log.Util().Debug("Relay removed from subscription", "sub_id", s.ID, "relay", url)
	return nil
}

// processMessages handles incoming messages for this subscription
func (s *Subscription) processMessages() {
	// TODO: This will be implemented to process messages from relay read handlers
	// For now, this is a placeholder that will be connected to relay message routing
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.Done:
			log.Util().Debug("Message processor stopped", "sub_id", s.ID)
			return
		case <-ticker.C:
			// Periodic heartbeat - could be used for subscription health checks
			log.Util().Debug("Subscription heartbeat", "sub_id", s.ID)
		}
	}
}

// IsActive returns whether the subscription is currently active
func (s *Subscription) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// GetRelayCount returns the number of relays in this subscription
func (s *Subscription) GetRelayCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Relays)
}

// GetFilters returns a copy of the subscription filters
func (s *Subscription) GetFilters() []nostr.Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	filters := make([]nostr.Filter, len(s.Filters))
	copy(filters, s.Filters)
	return filters
}