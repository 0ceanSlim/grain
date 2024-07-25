package utils

import (
	"os"

	"gopkg.in/yaml.v2"
)

type RateLimitConfig struct {
	WsLimit    float64           `yaml:"ws_limit"`
	WsBurst    int               `yaml:"ws_burst"`
	EventLimit float64           `yaml:"event_limit"`
	EventBurst int               `yaml:"event_burst"`
	KindLimits []KindLimitConfig `yaml:"kind_limits"`
	CategoryLimits map[string]KindLimitConfig `yaml:"category_limits"`
}

type KindLimitConfig struct {
	Kind  int     `yaml:"kind"`
	Limit float64 `yaml:"limit"`
	Burst int     `yaml:"burst"`
}

type CategoryLimitConfig struct {
	Regular                  LimitBurst `yaml:"regular"`
	Replaceable              LimitBurst `yaml:"replaceable"`
	ParameterizedReplaceable LimitBurst `yaml:"parameterized_replaceable"`
	Ephemeral                LimitBurst `yaml:"ephemeral"`
}

type LimitBurst struct {
	Limit float64 `yaml:"limit"`
	Burst int     `yaml:"burst"`
}

type Config struct {
	MongoDB struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongodb"`
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
