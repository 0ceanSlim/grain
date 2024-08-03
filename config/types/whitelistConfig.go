package config

type WhitelistConfig struct {
	Enabled bool     `yaml:"enabled"`
	Pubkeys []string `yaml:"pubkeys"`
	Npubs   []string `yaml:"npubs"`
}
