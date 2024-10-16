package config

type WhitelistConfig struct {
	PubkeyWhitelist struct {
		Enabled bool     `yaml:"enabled"`
		Pubkeys []string `yaml:"pubkeys"`
		Npubs   []string `yaml:"npubs"`
	} `yaml:"pubkey_whitelist"`

	KindWhitelist struct {
		Enabled bool     `yaml:"enabled"`
		Kinds   []string `yaml:"kinds"`
	} `yaml:"kind_whitelist"`

	DomainWhitelist struct {
		Enabled bool     `yaml:"enabled"`
		Domains []string `yaml:"domains"`
	} `yaml:"domain_whitelist"`
}
