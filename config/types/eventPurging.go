package config

type EventPurgeConfig struct {
	Enabled              bool            `yaml:"enabled" json:"enabled"`
	DisableAtStartup     bool            `yaml:"disable_at_startup" json:"disable_at_startup"`
	KeepIntervalHours    int             `yaml:"keep_interval_hours" json:"keep_interval_hours"`
	PurgeIntervalMinutes int             `yaml:"purge_interval_minutes" json:"purge_interval_minutes"`
	PurgeByCategory      map[string]bool `yaml:"purge_by_category" json:"purge_by_category"`
	PurgeByKindEnabled   bool            `yaml:"purge_by_kind_enabled" json:"purge_by_kind_enabled"`
	KindsToPurge         []int           `yaml:"kinds_to_purge" json:"kinds_to_purge"`
	ExcludeWhitelisted   bool            `yaml:"exclude_whitelisted" json:"exclude_whitelisted"`
}
