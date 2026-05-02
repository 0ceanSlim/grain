package relay

import (
	"strings"
	"time"
)

// Filter represents the criteria used to query events
type Filter struct {
	IDs     []string            `json:"ids,omitempty"`
	Authors []string            `json:"authors,omitempty"`
	Kinds   []int               `json:"kinds,omitempty"`
	Tags    map[string][]string `json:"#,omitempty"` // Fixed: should be "#" for tag filters
	Since   *time.Time          `json:"since,omitempty"`
	Until   *time.Time          `json:"until,omitempty"`
	Limit   *int                `json:"limit,omitempty"`
	Search  string              `json:"search,omitempty"` // NIP-50: fulltext search query
}

// MatchesEvent returns true if the event satisfies all filter criteria per NIP-01.
// An empty/zero field means "match all" for that field.
func (f Filter) MatchesEvent(evt Event) bool {
	// Check IDs (prefix match per NIP-01)
	if len(f.IDs) > 0 {
		matched := false
		for _, prefix := range f.IDs {
			if len(evt.ID) >= len(prefix) && evt.ID[:len(prefix)] == prefix {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check Authors (prefix match per NIP-01)
	if len(f.Authors) > 0 {
		matched := false
		for _, prefix := range f.Authors {
			if len(evt.PubKey) >= len(prefix) && evt.PubKey[:len(prefix)] == prefix {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check Kinds
	if len(f.Kinds) > 0 {
		matched := false
		for _, k := range f.Kinds {
			if evt.Kind == k {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check time range
	evtTime := time.Unix(evt.CreatedAt, 0)
	if f.Since != nil && evtTime.Before(*f.Since) {
		return false
	}
	if f.Until != nil && evtTime.After(*f.Until) {
		return false
	}

	// NIP-50: substring match on event content. nostrdb's index is
	// ingest-time, but BroadcastEvent calls MatchesEvent against
	// in-memory subscriptions before reindex completes — so we need
	// our own check here for live (post-EOSE) search subscriptions.
	// This is a substring match, not the tokenized AND-of-words match
	// nostrdb does at REQ time; consistent with NIP-50's "implementation-
	// defined" search semantics.
	if f.Search != "" {
		if !strings.Contains(strings.ToLower(evt.Content), strings.ToLower(f.Search)) {
			return false
		}
	}

	// Check tag filters (e.g. Tags["e"] = ["abc..."] means #e tag must contain "abc...")
	for tagName, filterValues := range f.Tags {
		if len(filterValues) == 0 {
			continue
		}
		// Collect all values for this tag from the event
		eventTagValues := make(map[string]struct{})
		for _, tag := range evt.Tags {
			if len(tag) >= 2 && tag[0] == tagName {
				eventTagValues[tag[1]] = struct{}{}
			}
		}
		// At least one filter value must be present in event tags
		matched := false
		for _, fv := range filterValues {
			if _, ok := eventTagValues[fv]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// ToSubscriptionFilter converts Filter to a relay-compatible format
func (f Filter) ToSubscriptionFilter() map[string]interface{} {
	filter := make(map[string]interface{})

	if len(f.IDs) > 0 {
		filter["ids"] = f.IDs
	}
	if len(f.Authors) > 0 {
		filter["authors"] = f.Authors
	}
	if len(f.Kinds) > 0 {
		filter["kinds"] = f.Kinds
	}
	if len(f.Tags) > 0 {
		for key, value := range f.Tags {
			filter["#"+key] = value
		}
	}
	if f.Since != nil {
		filter["since"] = f.Since.Unix()
	}
	if f.Until != nil {
		filter["until"] = f.Until.Unix()
	}
	if f.Limit != nil {
		filter["limit"] = *f.Limit
	}
	if f.Search != "" {
		filter["search"] = f.Search
	}

	return filter
}
