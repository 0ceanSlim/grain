package config

type CategoryLimitConfig struct {
	Regular     LimitBurst `yaml:"regular"`
	Replaceable LimitBurst `yaml:"replaceable"`
	Addressable LimitBurst `yaml:"addressable"`
	Ephemeral   LimitBurst `yaml:"ephemeral"`
}
