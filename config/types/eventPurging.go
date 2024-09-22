package config

type EventPurgeConfig struct {
	Enabled            bool            `yaml:"enabled"`
	KeepDurationDays   int             `yaml:"keep_duration_days"`
	PurgeIntervalHours int             `yaml:"purge_interval_hours"`
	PurgeByCategory    map[string]bool `yaml:"purge_by_category"`
	PurgeByKind        []KindPurgeRule `yaml:"purge_by_kind"`
	ExcludeWhitelisted bool            `yaml:"exclude_whitelisted"`
}

type KindPurgeRule struct {
	Kind    int  `yaml:"kind"`
	Enabled bool `yaml:"enabled"`
}
