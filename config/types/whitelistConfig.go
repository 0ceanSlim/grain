package config

// WhitelistConfig — whitelist.yml shape. JSON tags mirror the YAML
// ones so admin write methods (NIP-86 grain_updatewhitelistconfig)
// can round-trip the struct through JSON without losing snake_case
// field names.
type WhitelistConfig struct {
	PubkeyWhitelist struct {
		Enabled             bool     `yaml:"enabled" json:"enabled"`
		Pubkeys             []string `yaml:"pubkeys" json:"pubkeys"`
		Npubs               []string `yaml:"npubs" json:"npubs"`
		CacheRefreshMinutes int      `yaml:"cache_refresh_minutes" json:"cache_refresh_minutes"`
	} `yaml:"pubkey_whitelist" json:"pubkey_whitelist"`

	KindWhitelist struct {
		Enabled bool     `yaml:"enabled" json:"enabled"`
		Kinds   []string `yaml:"kinds" json:"kinds"`
	} `yaml:"kind_whitelist" json:"kind_whitelist"`

	DomainWhitelist struct {
		Enabled             bool     `yaml:"enabled" json:"enabled"`
		Domains             []string `yaml:"domains" json:"domains"`
		CacheRefreshMinutes int      `yaml:"cache_refresh_minutes" json:"cache_refresh_minutes"`
	} `yaml:"domain_whitelist" json:"domain_whitelist"`
}
