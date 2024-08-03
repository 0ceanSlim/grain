package config

type PubkeyWhitelistConfig struct {
	Enabled bool     `yaml:"enabled"`
	Pubkeys []string `yaml:"pubkeys"`
	Npubs   []string `yaml:"npubs"`
}
