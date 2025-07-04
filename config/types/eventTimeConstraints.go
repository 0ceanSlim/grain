package config

type EventTimeConstraints struct {
	MinCreatedAt       int64  `yaml:"min_created_at"`        // Minimum allowed timestamp
	MinCreatedAtString string `yaml:"min_created_at_string"` // Original string value for parsing (e.g., "now-5m")
	MaxCreatedAt       int64  `yaml:"max_created_at"`        // Maximum allowed timestamp
	MaxCreatedAtString string `yaml:"max_created_at_string"` // Original string value for parsing (e.g., "now+5m")
}
