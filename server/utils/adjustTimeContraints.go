package utils

import (
	"strings"
	"time"

	cfgTypes "github.com/0ceanslim/grain/config/types"
)

// AdjustEventTimeConstraints adjusts the event time constraints based on the configuration
func AdjustEventTimeConstraints(cfg *cfgTypes.ServerConfig) {
	now := time.Now()

	// Adjust min_created_at based on string value or default to January 1, 2020
	if strings.HasPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now") {
		offset := strings.TrimPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now")
		duration, err := time.ParseDuration(offset)
		if err != nil {
			utilLog().Error("Invalid time offset for min_created_at", "offset", offset, "error", err)
			cfg.EventTimeConstraints.MinCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
		} else {
			cfg.EventTimeConstraints.MinCreatedAt = now.Add(duration).Unix()
			utilLog().Debug("Min created_at adjusted with offset", 
				"offset", offset, 
				"timestamp", cfg.EventTimeConstraints.MinCreatedAt,
				"formatted_time", time.Unix(cfg.EventTimeConstraints.MinCreatedAt, 0).Format(time.RFC3339))
		}
	} else if cfg.EventTimeConstraints.MinCreatedAt == 0 {
		// Default to January 1, 2020, if not set
		cfg.EventTimeConstraints.MinCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
		utilLog().Debug("Min created_at set to default", 
			"timestamp", cfg.EventTimeConstraints.MinCreatedAt,
			"formatted_time", time.Unix(cfg.EventTimeConstraints.MinCreatedAt, 0).Format(time.RFC3339))
	}

	// Adjust max_created_at based on string value or default to current time
	if strings.HasPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now") {
		offset := strings.TrimPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now")
		duration, err := time.ParseDuration(offset)
		if err != nil {
			utilLog().Error("Invalid time offset for max_created_at", "offset", offset, "error", err)
			cfg.EventTimeConstraints.MaxCreatedAt = now.Unix() // Default to now if parsing fails
		} else {
			cfg.EventTimeConstraints.MaxCreatedAt = now.Add(duration).Unix()
			utilLog().Debug("Max created_at adjusted with offset", 
				"offset", offset, 
				"timestamp", cfg.EventTimeConstraints.MaxCreatedAt,
				"formatted_time", time.Unix(cfg.EventTimeConstraints.MaxCreatedAt, 0).Format(time.RFC3339))
		}
	} else if cfg.EventTimeConstraints.MaxCreatedAt == 0 {
		// Default to the current time if it's set to zero and no "now" keyword is used
		cfg.EventTimeConstraints.MaxCreatedAt = now.Unix()
		utilLog().Debug("Max created_at set to current time", 
			"timestamp", cfg.EventTimeConstraints.MaxCreatedAt,
			"formatted_time", time.Unix(cfg.EventTimeConstraints.MaxCreatedAt, 0).Format(time.RFC3339))
	}

	utilLog().Info("Event time constraints adjusted", 
		"min_timestamp", cfg.EventTimeConstraints.MinCreatedAt,
		"min_time", time.Unix(cfg.EventTimeConstraints.MinCreatedAt, 0).Format(time.RFC3339),
		"max_timestamp", cfg.EventTimeConstraints.MaxCreatedAt,
		"max_time", time.Unix(cfg.EventTimeConstraints.MaxCreatedAt, 0).Format(time.RFC3339))
}