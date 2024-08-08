package config

type ServerConfig struct {
	MongoDB struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongodb"`
	Server struct {
		Port         string `yaml:"port"`
		ReadTimeout  int    `yaml:"read_timeout"`  // Timeout in seconds
		WriteTimeout int    `yaml:"write_timeout"` // Timeout in seconds
		IdleTimeout  int    `yaml:"idle_timeout"`  // Timeout in seconds
	} `yaml:"server"`
	RateLimit       RateLimitConfig       `yaml:"rate_limit"`
	PubkeyWhitelist PubkeyWhitelistConfig `yaml:"pubkey_whitelist"`
	KindWhitelist   KindWhitelistConfig   `yaml:"kind_whitelist"`
	DomainWhitelist DomainWhitelistConfig `yaml:"domain_whitelist"`
}
