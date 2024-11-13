package config

type EventPurgeConfig struct {
	Enabled              bool            `yaml:"enabled"`
	KeepIntervalHours    int             `yaml:"keep_interval_hours"`
	PurgeIntervalMinutes int             `yaml:"purge_interval_minutes"`
	PurgeByCategory      map[string]bool `yaml:"purge_by_category"`
	PurgeByKindEnabled   bool            `yaml:"purge_by_kind_enabled"`
	KindsToPurge         []int           `yaml:"kinds_to_purge"`
	ExcludeWhitelisted   bool            `yaml:"exclude_whitelisted"`
}
