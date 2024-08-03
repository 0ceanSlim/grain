package config

type KindWhitelistConfig struct {
	Enabled bool     `yaml:"enabled"`
	Kinds   []string `yaml:"kinds"`
}
