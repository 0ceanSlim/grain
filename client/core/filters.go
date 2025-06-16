package core

import (
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// FilterBuilder provides a fluent interface for building Nostr filters
type FilterBuilder struct {
	filter nostr.Filter
}

// NewFilterBuilder creates a new filter builder
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{
		filter: nostr.Filter{},
	}
}

// Authors sets the authors filter
func (fb *FilterBuilder) Authors(pubkeys ...string) *FilterBuilder {
	fb.filter.Authors = append(fb.filter.Authors, pubkeys...)
	return fb
}

// Kinds sets the kinds filter
func (fb *FilterBuilder) Kinds(kinds ...int) *FilterBuilder {
	fb.filter.Kinds = append(fb.filter.Kinds, kinds...)
	return fb
}

// Since sets the since timestamp filter
func (fb *FilterBuilder) Since(timestamp time.Time) *FilterBuilder {
	fb.filter.Since = &timestamp
	return fb
}

// Until sets the until timestamp filter
func (fb *FilterBuilder) Until(timestamp time.Time) *FilterBuilder {
	fb.filter.Until = &timestamp
	return fb
}

// Limit sets the limit filter
func (fb *FilterBuilder) Limit(limit int) *FilterBuilder {
	fb.filter.Limit = &limit
	return fb
}

// Tag adds a tag filter
func (fb *FilterBuilder) Tag(name string, values ...string) *FilterBuilder {
	if fb.filter.Tags == nil {
		fb.filter.Tags = make(map[string][]string)
	}
	
	// Ensure tag name has # prefix for consistency
	tagKey := name
	if len(tagKey) > 0 && tagKey[0] != '#' {
		tagKey = "#" + tagKey
	}
	
	fb.filter.Tags[tagKey] = append(fb.filter.Tags[tagKey], values...)
	return fb
}

// IDs sets the event IDs filter
func (fb *FilterBuilder) IDs(ids ...string) *FilterBuilder {
	fb.filter.IDs = append(fb.filter.IDs, ids...)
	return fb
}

// Build returns the constructed filter
func (fb *FilterBuilder) Build() nostr.Filter {
	return fb.filter
}

// Common filter presets

// ProfileFilter creates a filter for user profiles (kind 0)
func ProfileFilter(pubkey string) nostr.Filter {
	return NewFilterBuilder().
		Authors(pubkey).
		Kinds(0).
		Limit(1).
		Build()
}

// NotesFilter creates a filter for notes from specific authors
func NotesFilter(authors []string, limit int) nostr.Filter {
	builder := NewFilterBuilder().
		Kinds(1).
		Limit(limit)
	
	for _, author := range authors {
		builder.Authors(author)
	}
	
	return builder.Build()
}

// ReactionsFilter creates a filter for reactions to a specific event
func ReactionsFilter(eventID string) nostr.Filter {
	return NewFilterBuilder().
		Kinds(7).
		Tag("e", eventID).
		Build()
}

// RelayListFilter creates a filter for relay lists (kind 10002)
func RelayListFilter(pubkey string) nostr.Filter {
	return NewFilterBuilder().
		Authors(pubkey).
		Kinds(10002).
		Limit(1).
		Build()
}

// ContactListFilter creates a filter for contact lists (kind 3)
func ContactListFilter(pubkey string) nostr.Filter {
	return NewFilterBuilder().
		Authors(pubkey).
		Kinds(3).
		Limit(1).
		Build()
}

// TimeRangeFilter creates a filter for events within a specific time range
func TimeRangeFilter(since, until time.Time, kinds []int) nostr.Filter {
	builder := NewFilterBuilder().
		Since(since).
		Until(until)
	
	if len(kinds) > 0 {
		builder.Kinds(kinds...)
	}
	
	return builder.Build()
}

// RecentNotesFilter creates a filter for recent notes
func RecentNotesFilter(limit int, maxAge time.Duration) nostr.Filter {
	since := time.Now().Add(-maxAge)
	
	return NewFilterBuilder().
		Kinds(1).
		Since(since).
		Limit(limit).
		Build()
}