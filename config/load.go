package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

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

type LimitBurst struct {
	Limit float64 `yaml:"limit"`
	Burst int     `yaml:"burst"`
}

type KindSizeLimitConfig struct {
	Kind    int `yaml:"kind"`
	MaxSize int `yaml:"max_size"`
}

type RateLimitConfig struct {
	WsLimit    float64           `yaml:"ws_limit"`
	WsBurst    int               `yaml:"ws_burst"`
	EventLimit float64           `yaml:"event_limit"`
	EventBurst int               `yaml:"event_burst"`
	ReqLimit float64             `yaml:"req_limit"`
	ReqBurst int               	 `yaml:"req_burst"`
	MaxEventSize   int                          `yaml:"max_event_size"`
	KindSizeLimits []KindSizeLimitConfig        `yaml:"kind_size_limits"`
	CategoryLimits map[string]KindLimitConfig `yaml:"category_limits"`
	KindLimits []KindLimitConfig `yaml:"kind_limits"`
}

type CategoryLimitConfig struct {
	Regular                  LimitBurst `yaml:"regular"`
	Replaceable              LimitBurst `yaml:"replaceable"`
	ParameterizedReplaceable LimitBurst `yaml:"parameterized_replaceable"`
	Ephemeral                LimitBurst `yaml:"ephemeral"`
}

type KindLimitConfig struct {
	Kind  int     `yaml:"kind"`
	Limit float64 `yaml:"limit"`
	Burst int     `yaml:"burst"`
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