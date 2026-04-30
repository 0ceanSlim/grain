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

type ServerConfig struct {
	Logging  LogConfig `yaml:"logging"`
	Database struct {
		Path      string `yaml:"path"`        // Directory for nostrdb data files (default: ./data)
		MapSizeMB int    `yaml:"map_size_mb"` // Max database size in MB (default: 4096 = 4GB)
	} `yaml:"database"`
	Server struct {
		Port                      string `yaml:"port"`
		ReadTimeout               int    `yaml:"read_timeout"`
		WriteTimeout              int    `yaml:"write_timeout"`
		IdleTimeout               int    `yaml:"idle_timeout"`
		MaxConnections            int    `yaml:"max_connections"`
		MaxSubscriptionsPerClient int    `yaml:"max_subscriptions_per_client"`
		ImplicitReqLimit          int    `yaml:"implicit_req_limit"`           // New field for implicit REQ limit
		ConnectionRateLimitPerIP  int    `yaml:"connection_rate_limit_per_ip"` // Per-IP connection attempts per minute (0 = disabled). See #61.
	} `yaml:"server"`
	Client               ClientConfig         `yaml:"client"`
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
}
