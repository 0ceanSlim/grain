package config

type LimitBurst struct {
	Limit float64 `yaml:"limit"`
	Burst int     `yaml:"burst"`
}