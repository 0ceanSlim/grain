package config

import (
	"os"
	"sync"

	configTypes "grain/config/types"

	"gopkg.in/yaml.v2"
)

var (
	cfg  *configTypes.ServerConfig
	once sync.Once
)

func LoadConfig(filename string) (*configTypes.ServerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config configTypes.ServerConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	once.Do(func() {
		cfg = &config
	})

	return cfg, nil
}

func GetConfig() *configTypes.ServerConfig {
	return cfg
}
