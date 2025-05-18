package config

type EventTimeConstraints struct {
	MinCreatedAt       int64  `yaml:"min_created_at"`        // Minimum allowed timestamp
	MinCreatedAtString string `yaml:"min_created_at_string"` // Original string value for parsing (e.g., "now-5m")
	MaxCreatedAt       int64  `yaml:"max_created_at"`        // Maximum allowed timestamp
	MaxCreatedAtString string `yaml:"max_created_at_string"` // Original string value for parsing (e.g., "now+5m")
}

type UserSyncConfig struct {
	UserSync              bool     `yaml:"user_sync"`               // Enable/disable syncing
	DisableAtStartup      bool     `yaml:"disable_at_startup"`      // New field
	InitialSyncRelays     []string `yaml:"initial_sync_relays"`     // Relays for initial kind10002 fetch
	Kinds                 []int    `yaml:"kinds"`                   // Kinds to sync
	Categories            string   `yaml:"categories"`              // Categories to sync
	Limit                 *int     `yaml:"limit"`                   // Limit per kind
	ExcludeNonWhitelisted bool     `yaml:"exclude_non_whitelisted"` // Sync only whitelisted users
	Interval              int      `yaml:"interval"`                // Resync interval in hours
}

type LogConfig struct {
	Level     string `yaml:"level"`
	File      string `yaml:"file"`
	MaxSizeMB int    `yaml:"max_log_size_mb"`
	Structure bool   `yaml:"structure"`
}

type ServerConfig struct {
	Logging LogConfig `yaml:"logging"`
	MongoDB struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongodb"`
	Server struct {
		Port                      string `yaml:"port"`
		ReadTimeout               int    `yaml:"read_timeout"`
		WriteTimeout              int    `yaml:"write_timeout"`
		IdleTimeout               int    `yaml:"idle_timeout"`
		MaxConnections            int    `yaml:"max_connections"`
		MaxSubscriptionsPerClient int    `yaml:"max_subscriptions_per_client"`
		ImplicitReqLimit          int    `yaml:"implicit_req_limit"` // New field for implicit REQ limit
	} `yaml:"server"`
	RateLimit            RateLimitConfig      `yaml:"rate_limit"`
	Blacklist            BlacklistConfig      `yaml:"blacklist"`
	ResourceLimits       ResourceLimits       `yaml:"resource_limits"`
	Auth                 AuthConfig           `yaml:"auth"`
	EventPurge           EventPurgeConfig     `yaml:"event_purge"`
	EventTimeConstraints EventTimeConstraints `yaml:"event_time_constraints"` // Added this field
	BackupRelay          struct {
		Enabled bool   `yaml:"enabled"`
		URL     string `yaml:"url"`
	} `yaml:"backup_relay"`
	UserSync UserSyncConfig `yaml:"UserSync"`
}
