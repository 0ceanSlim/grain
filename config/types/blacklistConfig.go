package config

// BlacklistConfig — blacklist.yml shape, plus the IP-related fields
// that live in config.yml's blacklist: section (LoadIPBlocklist
// merges both sources at startup). JSON tags mirror the YAML names
// so NIP-86 grain_updateblacklistconfig can round-trip the struct
// without losing field names.
type BlacklistConfig struct {
	Enabled                     bool     `yaml:"enabled" json:"enabled"`
	PermanentBanWords           []string `yaml:"permanent_ban_words" json:"permanent_ban_words"`
	TempBanWords                []string `yaml:"temp_ban_words" json:"temp_ban_words"`
	MaxTempBans                 int      `yaml:"max_temp_bans" json:"max_temp_bans"`
	TempBanDuration             int      `yaml:"temp_ban_duration" json:"temp_ban_duration"`
	PermanentBlacklistPubkeys   []string `yaml:"permanent_blacklist_pubkeys" json:"permanent_blacklist_pubkeys"`
	PermanentBlacklistNpubs     []string `yaml:"permanent_blacklist_npubs" json:"permanent_blacklist_npubs"`
	MuteListAuthors             []string `yaml:"mutelist_authors" json:"mutelist_authors"`
	MutelistCacheRefreshMinutes int      `yaml:"mutelist_cache_refresh_minutes" json:"mutelist_cache_refresh_minutes"`

	// IP blacklist fields (#62). Mirror the pubkey escalation pattern at
	// the network layer. CIDR-aware so a single entry can cover a /24.
	PermanentBlockedIPs      []string `yaml:"permanent_blocked_ips" json:"permanent_blocked_ips"`             // CIDR ("203.0.113.0/24") or single IP (parsed as /32 or /128)
	IPMaxTempBans            int      `yaml:"ip_max_temp_bans" json:"ip_max_temp_bans"`                       // temp bans accumulated before promotion to permanent
	IPTempBanDuration        int      `yaml:"ip_temp_ban_duration" json:"ip_temp_ban_duration"`               // seconds; how long a temp ban lasts
	IPRateViolationThreshold int      `yaml:"ip_rate_violation_threshold" json:"ip_rate_violation_threshold"` // rate-limit violations before triggering one temp ban
}
