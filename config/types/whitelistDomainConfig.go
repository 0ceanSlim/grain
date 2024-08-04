package config

type DomainWhitelistConfig struct {
	Enabled bool     `yaml:"enabled"`
	Domains []string `yaml:"domains"`
}
