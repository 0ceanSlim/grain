package config

type LogConfig struct {
	Level              string   `yaml:"level"`
	File               string   `yaml:"file"`
	MaxSizeMB          int      `yaml:"max_log_size_mb"`
	Structure          bool     `yaml:"structure"`
	CheckIntervalMin   int      `yaml:"check_interval_min"`  // How often the program checks the size of the current log file
	BackupCount        int      `yaml:"backup_count"`        // Number of backup logs to keep
	SuppressComponents []string `yaml:"suppress_components"` // Components to suppress INFO/DEBUG logs from
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
