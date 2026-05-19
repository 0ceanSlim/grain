package config

type LimitBurst struct {
	Limit float64 `yaml:"limit" json:"limit"`
	Burst int     `yaml:"burst" json:"burst"`
}
