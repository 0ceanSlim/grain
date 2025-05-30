package config

type WhitelistConfig struct {
	PubkeyWhitelist struct {
		Enabled             bool     `yaml:"enabled"`
		Pubkeys             []string `yaml:"pubkeys"`
		Npubs               []string `yaml:"npubs"`
		CacheRefreshMinutes int      `yaml:"cache_refresh_minutes"`
	} `yaml:"pubkey_whitelist"`

	KindWhitelist struct {
		Enabled bool     `yaml:"enabled"`
		Kinds   []string `yaml:"kinds"`
	} `yaml:"kind_whitelist"`

	DomainWhitelist struct {
		Enabled             bool     `yaml:"enabled"`
		Domains             []string `yaml:"domains"`
		CacheRefreshMinutes int      `yaml:"cache_refresh_minutes"`
	} `yaml:"domain_whitelist"`
}
