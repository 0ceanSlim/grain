package core

import (
	"fmt"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// EventBuilder provides a fluent interface for building Nostr events
type EventBuilder struct {
	kind      int
	content   string
	tags      [][]string
	createdAt *time.Time
}

// NewEventBuilder creates a new event builder with the specified kind
func NewEventBuilder(kind int) *EventBuilder {
	return &EventBuilder{
		kind: kind,
		tags: make([][]string, 0),
	}
}

// Content sets the content of the event
func (eb *EventBuilder) Content(content string) *EventBuilder {
	eb.content = content
	return eb
}

// Tag adds a generic tag to the event
func (eb *EventBuilder) Tag(name string, values ...string) *EventBuilder {
	tag := make([]string, len(values)+1)
	tag[0] = name
	copy(tag[1:], values)
	eb.tags = append(eb.tags, tag)
	
	log.Util().Debug("Added tag to event", "tag_name", name, "values", values)
	return eb
}

// PTag adds a 'p' tag (pubkey reference) to the event
func (eb *EventBuilder) PTag(pubkey string, relayHint ...string) *EventBuilder {
	tag := []string{"p", pubkey}
	if len(relayHint) > 0 && relayHint[0] != "" {
		tag = append(tag, relayHint[0])
	}
	eb.tags = append(eb.tags, tag)
	
	log.Util().Debug("Added p tag to event", "pubkey", pubkey)
	return eb
}

// ETag adds an 'e' tag (event reference) to the event
func (eb *EventBuilder) ETag(eventID string, relayHint, marker string) *EventBuilder {
	tag := []string{"e", eventID}
	
	if relayHint != "" {
		tag = append(tag, relayHint)
		if marker != "" {
			tag = append(tag, marker)
		}
	} else if marker != "" {
		// If we have a marker but no relay hint, add empty string for relay
		tag = append(tag, "", marker)
	}
	
	eb.tags = append(eb.tags, tag)
	
	log.Util().Debug("Added e tag to event", "event_id", eventID, "marker", marker)
	return eb
}

// RTag adds an 'r' tag (relay reference) to the event
func (eb *EventBuilder) RTag(relayURL string, marker string) *EventBuilder {
	tag := []string{"r", relayURL}
	if marker != "" {
		tag = append(tag, marker)
	}
	eb.tags = append(eb.tags, tag)
	
	log.Util().Debug("Added r tag to event", "relay", relayURL, "marker", marker)
	return eb
}

// DTag adds a 'd' tag (identifier) to the event
func (eb *EventBuilder) DTag(identifier string) *EventBuilder {
	eb.tags = append(eb.tags, []string{"d", identifier})
	
	log.Util().Debug("Added d tag to event", "identifier", identifier)
	return eb
}

// ATag adds an 'a' tag (address reference) to the event
func (eb *EventBuilder) ATag(kind int, pubkey string, dTag string, relayHint ...string) *EventBuilder {
	coordinate := fmt.Sprintf("%d:%s:%s", kind, pubkey, dTag)
	tag := []string{"a", coordinate}
	
	if len(relayHint) > 0 && relayHint[0] != "" {
		tag = append(tag, relayHint[0])
	}
	
	eb.tags = append(eb.tags, tag)
	
	log.Util().Debug("Added a tag to event", "coordinate", coordinate)
	return eb
}

// TTag adds a 't' tag (hashtag) to the event
func (eb *EventBuilder) TTag(hashtag string) *EventBuilder {
	eb.tags = append(eb.tags, []string{"t", hashtag})
	
	log.Util().Debug("Added t tag to event", "hashtag", hashtag)
	return eb
}

// CreatedAt sets the created_at timestamp for the event
func (eb *EventBuilder) CreatedAt(t time.Time) *EventBuilder {
	eb.createdAt = &t
	return eb
}

// Build constructs the final Event struct (without signing)
func (eb *EventBuilder) Build() *nostr.Event {
	event := &nostr.Event{
		Kind:    eb.kind,
		Content: eb.content,
		Tags:    eb.tags,
	}
	
	// Set timestamp
	if eb.createdAt != nil {
		event.CreatedAt = eb.createdAt.Unix()
	} else {
		event.CreatedAt = time.Now().Unix()
	}
	
	log.Util().Debug("Built event", 
		"kind", event.Kind,
		"content_length", len(event.Content),
		"tag_count", len(event.Tags),
		"created_at", event.CreatedAt)
	
	return event
}

// Common event builder presets

// NewTextNote creates a builder for a text note (kind 1)
func NewTextNote(content string) *EventBuilder {
	return NewEventBuilder(1).Content(content)
}

// NewReaction creates a builder for a reaction (kind 7)
func NewReaction(eventID string, content string) *EventBuilder {
	return NewEventBuilder(7).
		Content(content).
		ETag(eventID, "", "")
}

// NewRepost creates a builder for a repost (kind 6)
func NewRepost(eventID string, relayHint string) *EventBuilder {
	return NewEventBuilder(6).
		ETag(eventID, relayHint, "")
}

// NewDeletion creates a builder for a deletion event (kind 5)
func NewDeletion(eventIDs []string, reason string) *EventBuilder {
	builder := NewEventBuilder(5).Content(reason)
	
	for _, eventID := range eventIDs {
		builder.ETag(eventID, "", "")
	}
	
	return builder
}

// NewContactList creates a builder for a contact list (kind 3)
func NewContactList() *EventBuilder {
	return NewEventBuilder(3)
}

// NewRelayList creates a builder for a relay list (kind 10002)
func NewRelayList() *EventBuilder {
	return NewEventBuilder(10002)
}

// NewProfile creates a builder for a profile event (kind 0)
func NewProfile() *EventBuilder {
	return NewEventBuilder(0)
}