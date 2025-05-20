package validation

import (
	"strings"
	"time"

	configTypes "github.com/0ceanslim/grain/config/types"
	relay "github.com/0ceanslim/grain/server/types"
)

func ValidateEventTimestamp(evt relay.Event, cfg *configTypes.ServerConfig) bool {
    if cfg == nil {
        validationLog().Error("Server configuration is not loaded")
        return false
    }

    now := time.Now()
    var minCreatedAt, maxCreatedAt int64

    // Dynamically calculate min_created_at based on string configuration
    if strings.HasPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now") {
        offset := strings.TrimPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now")
        duration, err := time.ParseDuration(offset)
        if err != nil {
            validationLog().Error("Invalid time offset for min_created_at", "offset", offset, "error", err)
            minCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
        } else {
            minCreatedAt = now.Add(duration).Unix()
        }
    } else if cfg.EventTimeConstraints.MinCreatedAt == 0 {
        // Use January 1, 2020, as the default minimum timestamp
        minCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
    } else {
        minCreatedAt = cfg.EventTimeConstraints.MinCreatedAt
    }

    // Dynamically calculate max_created_at based on string configuration
    if strings.HasPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now") {
        offset := strings.TrimPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now")
        duration, err := time.ParseDuration(offset)
        if err != nil {
            validationLog().Error("Invalid time offset for max_created_at", "offset", offset, "error", err)
            maxCreatedAt = now.Unix() // Default to now if parsing fails
        } else {
            maxCreatedAt = now.Add(duration).Unix()
        }
    } else if cfg.EventTimeConstraints.MaxCreatedAt == 0 {
        // Default to the current time if not set
        maxCreatedAt = now.Unix()
    } else {
        maxCreatedAt = cfg.EventTimeConstraints.MaxCreatedAt
    }

    // Check if the event's created_at timestamp falls within the allowed range
    if evt.CreatedAt < minCreatedAt || evt.CreatedAt > maxCreatedAt {
        validationLog().Warn("Event timestamp out of range", 
            "event_id", evt.ID, 
            "timestamp", evt.CreatedAt, 
            "min", minCreatedAt, 
            "max", maxCreatedAt,
            "event_time", time.Unix(evt.CreatedAt, 0).Format(time.RFC3339),
            "min_time", time.Unix(minCreatedAt, 0).Format(time.RFC3339),
            "max_time", time.Unix(maxCreatedAt, 0).Format(time.RFC3339))
        return false
    }

    return true
}