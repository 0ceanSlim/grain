package config

type BlacklistConfig struct {
	Enabled                   bool     `yaml:"enabled"`
	PermanentBanWords         []string `yaml:"permanent_ban_words"`
	TempBanWords              []string `yaml:"temp_ban_words"`
	MaxTempBans               int      `yaml:"max_temp_bans"`
	TempBanDuration           int      `yaml:"temp_ban_duration"`
	PermanentBlacklistPubkeys []string `yaml:"permanent_blacklist_pubkeys"`
	PermanentBlacklistNpubs   []string `yaml:"permanent_blacklist_npubs"`
	MuteListAuthors           []string `yaml:"mutelist_authors"`
}