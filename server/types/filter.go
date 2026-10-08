package relay

import (
	"encoding/json"
	"strings"
	"time"
)

// Filter represents the criteria used to query events. Its JSON form is the
// NIP-01 wire format (see MarshalJSON), not the struct layout: tag filters are
// flattened to "#<name>" keys and since/until are unix seconds.
type Filter struct {
	IDs     []string `json:"ids,omitempty"`
	Authors []string `json:"authors,omitempty"`
	Kinds   []int    `json:"kinds,omitempty"`
	// Tags maps a tag name WITHOUT the leading '#' ("d", "p", "e") to the
	// values to match. A leading '#' is tolerated and stripped wherever the
	// filter is matched or encoded.
	Tags   map[string][]string `json:"-"`
	Since  *time.Time          `json:"since,omitempty"`
	Until  *time.Time          `json:"until,omitempty"`
	Limit  *int                `json:"limit,omitempty"`
	Search string              `json:"search,omitempty"` // NIP-50: fulltext search query
}

// TagFilterName normalizes a Tags key to the bare tag name: "#d" -> "d".
// A lone "#" is left alone since it is itself a (strange) tag name.
func TagFilterName(key string) string {
	if len(key) >= 2 && key[0] == '#' {
		return key[1:]
	}
	return key
}

// MarshalJSON encodes the filter in NIP-01 wire format. A nil list is omitted,
// but an empty non-nil list is kept: per NIP-01 it matches nothing, and
// dropping it would widen the filter to match everything.
func (f Filter) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{}, 8)
	if f.IDs != nil {
		m["ids"] = f.IDs
	}
	if f.Authors != nil {
		m["authors"] = f.Authors
	}
	if f.Kinds != nil {
		m["kinds"] = f.Kinds
	}
	for key, values := range f.Tags {
		if values == nil {
			continue
		}
		m["#"+TagFilterName(key)] = values
	}
	if f.Since != nil {
		m["since"] = f.Since.Unix()
	}
	if f.Until != nil {
		m["until"] = f.Until.Unix()
	}
	if f.Limit != nil {
		m["limit"] = *f.Limit
	}
	if f.Search != "" {
		m["search"] = f.Search
	}
	return json.Marshal(m)
}

// UnmarshalJSON decodes a NIP-01 filter object with the same strict rules the
// relay applies to incoming REQs (see ParseFilter).
func (f *Filter) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, _, err := ParseFilter(raw)
	if err != nil {
		return err
	}
	*f = parsed
	return nil
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
	for key, filterValues := range f.Tags {
		if len(filterValues) == 0 {
			continue
		}
		tagName := TagFilterName(key)
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
	for key, value := range f.Tags {
		filter["#"+TagFilterName(key)] = value
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
