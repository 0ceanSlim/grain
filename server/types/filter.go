package relay

import "time"

// Filter represents the criteria used to query events
type Filter struct {
	IDs     []string            `json:"ids,omitempty"`
	Authors []string            `json:"authors,omitempty"`
	Kinds   []int               `json:"kinds,omitempty"`
	Tags    map[string][]string `json:"#,omitempty"` // Fixed: should be "#" for tag filters
	Since   *time.Time          `json:"since,omitempty"`
	Until   *time.Time          `json:"until,omitempty"`
	Limit   *int                `json:"limit,omitempty"`
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

	return filter
}
