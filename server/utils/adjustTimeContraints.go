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

	// Adjust min_created_at based on string value or default to January 1, 2020
	if strings.HasPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now") {
		offset := strings.TrimPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now")
		duration, err := time.ParseDuration(offset)
		if err != nil {
			fmt.Printf("Invalid time offset for min_created_at: %s\n", offset)
			cfg.EventTimeConstraints.MinCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
		} else {
			cfg.EventTimeConstraints.MinCreatedAt = now.Add(duration).Unix()
		}
	} else if cfg.EventTimeConstraints.MinCreatedAt == 0 {
		// Default to January 1, 2020, if not set
		cfg.EventTimeConstraints.MinCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	}

	// Adjust max_created_at based on string value or default to current time
	if strings.HasPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now") {
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