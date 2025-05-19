package validation

import (
	"time"

	configTypes "github.com/0ceanslim/grain/config/types"
	relay "github.com/0ceanslim/grain/server/types"
)

// ValidateEventTimestamp validates if an event's timestamp is within the allowed range
func ValidateEventTimestamp(evt relay.Event, cfg *configTypes.ServerConfig) bool {
	if cfg == nil {
		validationLog().Error("Server configuration is not loaded")
		return false
	}

	// Use current time for max and a fixed date for min if not specified
	now := time.Now().Unix()
	minCreatedAt := cfg.EventTimeConstraints.MinCreatedAt
	if minCreatedAt == 0 {
		// Use January 1, 2020, as the default minimum timestamp
		minCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	}

	maxCreatedAt := cfg.EventTimeConstraints.MaxCreatedAt
	if maxCreatedAt == 0 {
		// Default to the current time if not set
		maxCreatedAt = now
	}

	// Check if the event's created_at timestamp falls within the allowed range
	if evt.CreatedAt < minCreatedAt || evt.CreatedAt > maxCreatedAt {
		validationLog().Warn("Event timestamp out of range", 
			"event_id", evt.ID, 
			"timestamp", evt.CreatedAt, 
			"min", minCreatedAt, 
			"max", maxCreatedAt)
		return false
	}

	return true
}