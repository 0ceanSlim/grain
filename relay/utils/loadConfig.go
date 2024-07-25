package utils

import (
	"os"

	"gopkg.in/yaml.v2"
)

type RateLimitConfig struct {
	EventLimit float64           `yaml:"event_limit"`
	EventBurst int               `yaml:"event_burst"`
	WsLimit    float64           `yaml:"ws_limit"`
	WsBurst    int               `yaml:"ws_burst"`
	KindLimits []KindLimitConfig `yaml:"kind_limits"`
}

type KindLimitConfig struct {
	Kind  int     `yaml:"kind"`
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
