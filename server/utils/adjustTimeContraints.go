package utils

import (
	"fmt"
	config "grain/config/types"
	"strings"
	"time"
)

// Adjusts the event time constraints based on the configuration
func AdjustEventTimeConstraints(cfg *config.ServerConfig) {
	now := time.Now()

	// Adjust min_created_at (no changes needed if it's already set in the config)
	if cfg.EventTimeConstraints.MinCreatedAt == 0 {
		cfg.EventTimeConstraints.MinCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	}

	// Adjust max_created_at
	if strings.HasPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now") {
		// Extract the offset (e.g., "+5m")
		offset := strings.TrimPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now")
		duration, err := time.ParseDuration(offset)
		if err != nil {
			fmt.Printf("Invalid time offset for max_created_at: %s\n", offset)
			cfg.EventTimeConstraints.MaxCreatedAt = now.Unix() // Default to now if parsing fails
		} else {
			cfg.EventTimeConstraints.MaxCreatedAt = now.Add(duration).Unix()
		}
	} else if cfg.EventTimeConstraints.MaxCreatedAt == 0 {
		// Default to the current time if it's set to zero and no "now" keyword is used
		cfg.EventTimeConstraints.MaxCreatedAt = now.Unix()
	}
}