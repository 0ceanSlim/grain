package config

type LogConfig struct {
	Level              string   `yaml:"level" json:"level"`
	File               string   `yaml:"file" json:"file"`
	MaxSizeMB          int      `yaml:"max_log_size_mb" json:"max_log_size_mb"`
	Structure          bool     `yaml:"structure" json:"structure"`
	Stdout             bool     `yaml:"stdout" json:"stdout"`                           // Mirror log records to stdout (pretty single-line format) so `docker logs` works without losing the file sink.
	CheckIntervalMin   int      `yaml:"check_interval_min" json:"check_interval_min"`   // How often the program checks the size of the current log file
	BackupCount        int      `yaml:"backup_count" json:"backup_count"`               // Number of backup logs to keep
	SuppressComponents []string `yaml:"suppress_components" json:"suppress_components"` // Components to suppress INFO/DEBUG logs from
}

// DatabaseConfig captures the nostrdb settings on disk. Pulled out
// of the anonymous struct inside ServerConfig so the NIP-86 admin
// write helpers can name a type (anonymous struct fields can't
// cross package boundaries cleanly).
type DatabaseConfig struct {
	Path      string `yaml:"path" json:"path"`               // Directory for nostrdb data files (default: ./data)
	MapSizeMB int    `yaml:"map_size_mb" json:"map_size_mb"` // Max database size in MB (default: 4096 = 4GB)
}

// ServerSettings is the HTTP server block (timeouts, connection
// caps, max subscriptions). Promoted to a named type for the same
// reason as DatabaseConfig.
type ServerSettings struct {
	Port                      string `yaml:"port" json:"port"`
	ReadTimeout               int    `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout              int    `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout               int    `yaml:"idle_timeout" json:"idle_timeout"`
	MaxConnections            int    `yaml:"max_connections" json:"max_connections"`
	MaxSubscriptionsPerClient int    `yaml:"max_subscriptions_per_client" json:"max_subscriptions_per_client"`
	ImplicitReqLimit          int    `yaml:"implicit_req_limit" json:"implicit_req_limit"`                     // New field for implicit REQ limit
	ConnectionRateLimitPerIP  int    `yaml:"connection_rate_limit_per_ip" json:"connection_rate_limit_per_ip"` // Per-IP connection attempts per minute (0 = disabled). See #61.
}

// BackupRelayConfig is the upstream-mirror destination. Same
// promotion reason — the NIP-86 update method needs a name to
// decode JSON into.
type BackupRelayConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	URL     string `yaml:"url" json:"url"`
}

type ServerConfig struct {
	Logging              LogConfig            `yaml:"logging" json:"logging"`
	Database             DatabaseConfig       `yaml:"database" json:"database"`
	Server               ServerSettings       `yaml:"server" json:"server"`
	Client               ClientConfig         `yaml:"client" json:"client"`
	RateLimit            RateLimitConfig      `yaml:"rate_limit" json:"rate_limit"`
	Blacklist            BlacklistConfig      `yaml:"blacklist" json:"blacklist"`
	ResourceLimits       ResourceLimits       `yaml:"resource_limits" json:"resource_limits"`
	Auth                 AuthConfig           `yaml:"auth" json:"auth"`
	EventPurge           EventPurgeConfig     `yaml:"event_purge" json:"event_purge"`
	EventTimeConstraints EventTimeConstraints `yaml:"event_time_constraints" json:"event_time_constraints"`
	BackupRelay          BackupRelayConfig    `yaml:"backup_relay" json:"backup_relay"`
}
