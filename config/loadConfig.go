package config

import (
	"os"

	config "grain/config/types"

	"gopkg.in/yaml.v2"
)

func LoadConfig(filename string) (*config.ServerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config config.ServerConfig

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}