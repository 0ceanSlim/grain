package config

type ClientConfig struct {
	// IndexRelays seed the client library's discovery: NIP-65 mailbox
	// lookups and profile metadata fetches go here first when no
	// per-user relay set is known. As the outbox-model pool work
	// progresses (#56), these become the bootstrap layer that
	// per-user mailbox/inbox sets are resolved through, rather than
	// the relay set used for every operation.
	IndexRelays       []string `yaml:"index_relays" json:"index_relays"`
	ConnectionTimeout int      `yaml:"connection_timeout" json:"connection_timeout"` // seconds
	ReadTimeout       int      `yaml:"read_timeout" json:"read_timeout"`             // seconds
	WriteTimeout      int      `yaml:"write_timeout" json:"write_timeout"`           // seconds
	MaxConnections    int      `yaml:"max_connections" json:"max_connections"`
	RetryAttempts     int      `yaml:"retry_attempts" json:"retry_attempts"`
	RetryDelay        int      `yaml:"retry_delay" json:"retry_delay"` // seconds
	KeepAlive         bool     `yaml:"keep_alive" json:"keep_alive"`
	UserAgent         string   `yaml:"user_agent" json:"user_agent"`

	// ClientTag controls grain's NIP-89 `client` tag on events it authors.
	// grain always strips any foreign `client` tag (e.g. a stale
	// `client:amethyst` carried over on a replaceable event); when enabled it
	// stamps its own `["client", name]`. On by default; a logged-in user can opt
	// out for their own events via the settings slider (#99).
	ClientTag ClientTagConfig `yaml:"client_tag" json:"client_tag"`
}

// ClientTagConfig is the server-side default + name for grain's client tag.
type ClientTagConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Name    string `yaml:"name" json:"name"`
}
