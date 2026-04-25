package config

type ClientConfig struct {
	// IndexRelays seed the client library's discovery: NIP-65 mailbox
	// lookups and profile metadata fetches go here first when no
	// per-user relay set is known. As the outbox-model pool work
	// progresses (#56), these become the bootstrap layer that
	// per-user mailbox/inbox sets are resolved through, rather than
	// the relay set used for every operation.
	IndexRelays       []string `yaml:"index_relays"`
	ConnectionTimeout int      `yaml:"connection_timeout"` // seconds
	ReadTimeout       int      `yaml:"read_timeout"`       // seconds
	WriteTimeout      int      `yaml:"write_timeout"`      // seconds
	MaxConnections    int      `yaml:"max_connections"`
	RetryAttempts     int      `yaml:"retry_attempts"`
	RetryDelay        int      `yaml:"retry_delay"` // seconds
	KeepAlive         bool     `yaml:"keep_alive"`
	UserAgent         string   `yaml:"user_agent"`
}
