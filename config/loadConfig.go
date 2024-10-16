package config

import (
	"os"
	"sync"

	configTypes "grain/config/types"

	"gopkg.in/yaml.v2"
)

var (
    cfg            *configTypes.ServerConfig
    whitelistCfg   *configTypes.WhitelistConfig
    once           sync.Once
    whitelistOnce  sync.Once
)

// LoadConfig loads the server configuration from config.yml
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

// LoadWhitelistConfig loads the whitelist configuration from whitelist.yml
func LoadWhitelistConfig(filename string) (*configTypes.WhitelistConfig, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    var config configTypes.WhitelistConfig
    err = yaml.Unmarshal(data, &config)
    if err != nil {
        return nil, err
    }

    whitelistOnce.Do(func() {
        whitelistCfg = &config
    })

    return whitelistCfg, nil
}

func GetConfig() *configTypes.ServerConfig {
    return cfg
}

func GetWhitelistConfig() *configTypes.WhitelistConfig {
    return whitelistCfg
}