// client/core/eventSerializer.go
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// SerializeEvent manually constructs the JSON string for event serialization according to NIP-01
func SerializeEvent(evt nostr.Event) string {
	eventData := []interface{}{
		0,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		evt.Tags,
		evt.Content,
	}
	
	// Use Go's standard JSON marshaling first
	jsonBytes, err := json.Marshal(eventData)
	if err != nil {
		log.Util().Error("Failed to serialize event", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey,
			"kind", evt.Kind,
			"error", err)
		return ""
	}
	
	// Convert to NIP-01 compliant format
	jsonStr := string(jsonBytes)
	jsonStr = normalizeJSONForNIP01(jsonStr)
	
	// Only log at debug level for very important events or when troubleshooting
	if evt.Kind == 0 || evt.Kind == 3 {
		log.Util().Debug("Event serialized", 
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size_bytes", len(jsonStr))
	}
	
	return jsonStr
}

// normalizeJSONForNIP01 converts Go's JSON output to NIP-01 compliant format
func normalizeJSONForNIP01(jsonStr string) string {
	// Go's json.Marshal escapes some characters that NIP-01 says should NOT be escaped
	// We need to unescape Unicode sequences like \u0026 back to their original form
	
	// Replace common Unicode escapes that Go adds but NIP-01 doesn't require
	replacements := map[string]string{
		"\\u0026": "&",  // Ampersand
		"\\u003c": "<",  // Less than
		"\\u003e": ">",  // Greater than
		"\\u003d": "=",  // Equals sign
		"\\u002b": "+",  // Plus sign
		"\\u0027": "'",  // Single quote (apostrophe)
		"\\u002f": "/",  // Forward slash
	}
	
	result := jsonStr
	for escaped, unescaped := range replacements {
		result = strings.ReplaceAll(result, escaped, unescaped)
	}
	
	return result
}

// DeserializeEvent deserializes JSON bytes to an event
func DeserializeEvent(data []byte) (*nostr.Event, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	
	var event nostr.Event
	if err := json.Unmarshal(data, &event); err != nil {
		log.Util().Error("Failed to deserialize event", "error", err)
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}
	
	log.Util().Debug("Event deserialized", "event_id", event.ID, "kind", event.Kind)
	return &event, nil
}

// ComputeEventID computes the event ID according to NIP-01
func ComputeEventID(event *nostr.Event) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event cannot be nil")
	}
	
	// NIP-01: Event ID is SHA256 of the serialized event array
	// [0, pubkey, created_at, kind, tags, content]
	serialized := SerializeEvent(*event) // Dereference pointer to pass value
	if serialized == "" {
		return "", fmt.Errorf("failed to serialize event for ID computation")
	}
	
	hash := sha256.Sum256([]byte(serialized))
	eventID := hex.EncodeToString(hash[:])
	
	log.Util().Debug("Computed event ID", "event_id", eventID, "kind", event.Kind)
	return eventID, nil
}

// ValidateEventStructure validates the basic structure of an event
func ValidateEventStructure(event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	
	// Validate required fields
	if event.PubKey == "" {
		return fmt.Errorf("event must have a pubkey")
	}
	
	if len(event.PubKey) != 64 {
		return fmt.Errorf("pubkey must be 64 hex characters")
	}
	
	if event.CreatedAt <= 0 {
		return fmt.Errorf("created_at must be positive")
	}
	
	if event.Kind < 0 {
		return fmt.Errorf("kind must be non-negative")
	}
	
	// Validate tags structure
	for i, tag := range event.Tags {
		if len(tag) == 0 {
			return fmt.Errorf("tag %d is empty", i)
		}
		if tag[0] == "" {
			return fmt.Errorf("tag %d has empty tag name", i)
		}
	}
	
	// Validate signature if present
	if event.Sig != "" {
		if len(event.Sig) != 128 {
			return fmt.Errorf("signature must be 128 hex characters")
		}
	}
	
	// Validate ID if present
	if event.ID != "" {
		if len(event.ID) != 64 {
			return fmt.Errorf("event ID must be 64 hex characters")
		}
		
		// Verify computed ID matches
		computedID, err := ComputeEventID(event)
		if err != nil {
			return fmt.Errorf("failed to compute ID for validation: %w", err)
		}
		
		if event.ID != computedID {
			return fmt.Errorf("event ID does not match computed ID")
		}
	}
	
	log.Util().Debug("Event structure validated", "event_id", event.ID, "kind", event.Kind)
	return nil
}

// EventToJSON converts an event to pretty-printed JSON
func EventToJSON(event *nostr.Event) ([]byte, error) {
	if event == nil {
		return nil, fmt.Errorf("event cannot be nil")
	}
	
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event to JSON: %w", err)
	}
	
	return data, nil
}

// EventFromJSON parses an event from JSON
func EventFromJSON(data []byte) (*nostr.Event, error) {
	return DeserializeEvent(data)
}

// SerializeEventArray serializes an event for inclusion in a Nostr message array
func SerializeEventArray(events []*nostr.Event) ([]byte, error) {
	if len(events) == 0 {
		return json.Marshal([]interface{}{})
	}
	
	eventArray := make([]interface{}, len(events))
	for i, event := range events {
		eventArray[i] = event
	}
	
	data, err := json.Marshal(eventArray)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event array: %w", err)
	}
	
	log.Util().Debug("Event array serialized", "event_count", len(events), "size_bytes", len(data))
	return data, nil
}

// CreateNostrMessage creates a properly formatted Nostr protocol message
func CreateNostrMessage(messageType string, args ...interface{}) ([]byte, error) {
	message := make([]interface{}, len(args)+1)
	message[0] = messageType
	copy(message[1:], args)
	
	data, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nostr message: %w", err)
	}
	
	log.Util().Debug("Nostr message created", "type", messageType, "size_bytes", len(data))
	return data, nil
}

// ParseNostrMessage parses a Nostr protocol message
func ParseNostrMessage(data []byte) (messageType string, args []interface{}, err error) {
	var message []interface{}
	
	if err := json.Unmarshal(data, &message); err != nil {
		return "", nil, fmt.Errorf("failed to parse message: %w", err)
	}
	
	if len(message) == 0 {
		return "", nil, fmt.Errorf("empty message")
	}
	
	messageType, ok := message[0].(string)
	if !ok {
		return "", nil, fmt.Errorf("message type must be string")
	}
	
	args = message[1:]
	
	log.Util().Debug("Nostr message parsed", "type", messageType, "arg_count", len(args))
	return messageType, args, nil
}