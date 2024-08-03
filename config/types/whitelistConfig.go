package config

type WhitelistConfig struct {
	Enabled bool     `yaml:"enabled"`
	Pubkeys []string `yaml:"pubkeys"`
}
