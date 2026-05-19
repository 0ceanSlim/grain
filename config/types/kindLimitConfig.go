package config

type KindLimitConfig struct {
	Kind  int     `yaml:"kind" json:"kind"`
	Limit float64 `yaml:"limit" json:"limit"`
	Burst int     `yaml:"burst" json:"burst"`
}
