package config

type AuthConfig struct {
	Required bool   `yaml:"required"`
	RelayURL string `yaml:"relay_url"`
}
