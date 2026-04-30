package config

type BlacklistConfig struct {
	Enabled                     bool     `yaml:"enabled"`
	PermanentBanWords           []string `yaml:"permanent_ban_words"`
	TempBanWords                []string `yaml:"temp_ban_words"`
	MaxTempBans                 int      `yaml:"max_temp_bans"`
	TempBanDuration             int      `yaml:"temp_ban_duration"`
	PermanentBlacklistPubkeys   []string `yaml:"permanent_blacklist_pubkeys"`
	PermanentBlacklistNpubs     []string `yaml:"permanent_blacklist_npubs"`
	MuteListAuthors             []string `yaml:"mutelist_authors"`
	MutelistCacheRefreshMinutes int      `yaml:"mutelist_cache_refresh_minutes"`

	// IP blacklist fields (#62). Mirror the pubkey escalation pattern at
	// the network layer. CIDR-aware so a single entry can cover a /24.
	PermanentBlockedIPs      []string `yaml:"permanent_blocked_ips"`       // CIDR ("203.0.113.0/24") or single IP (parsed as /32 or /128)
	IPMaxTempBans            int      `yaml:"ip_max_temp_bans"`            // temp bans accumulated before promotion to permanent
	IPTempBanDuration        int      `yaml:"ip_temp_ban_duration"`        // seconds; how long a temp ban lasts
	IPRateViolationThreshold int      `yaml:"ip_rate_violation_threshold"` // rate-limit violations before triggering one temp ban
}
