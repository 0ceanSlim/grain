package utils

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	MongoDB struct {
		URI      string `yaml:"uri"`
		Database string `yaml:"database"`
	} `yaml:"mongodb"`
	Relay struct {
		Port string `yaml:"port"`
	} `yaml:"relay"`
	Web struct {
		Port string `yaml:"port"`
	} `yaml:"web"`
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
