package config

type ServerConfig struct {
	MongoDB struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongodb"`
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Whitelist WhitelistConfig `yaml:"whitelist"`
}
