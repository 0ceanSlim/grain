package config

type ClientConfig struct {
	DefaultRelays     []string `yaml:"default_relays"`
	ConnectionTimeout int      `yaml:"connection_timeout"` // seconds
	ReadTimeout       int      `yaml:"read_timeout"`       // seconds
	WriteTimeout      int      `yaml:"write_timeout"`      // seconds
	MaxConnections    int      `yaml:"max_connections"`
	RetryAttempts     int      `yaml:"retry_attempts"`
	RetryDelay        int      `yaml:"retry_delay"` // seconds
	KeepAlive         bool     `yaml:"keep_alive"`
	UserAgent         string   `yaml:"user_agent"`
}
