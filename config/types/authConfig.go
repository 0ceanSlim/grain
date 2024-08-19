package config

type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	RelayURL string `yaml:"relay_url"`
}