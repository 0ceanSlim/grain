package config

type KindLimitConfig struct {
	Kind  int     `yaml:"kind"`
	Limit float64 `yaml:"limit"`
	Burst int     `yaml:"burst"`
}