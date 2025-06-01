package config

import (
	"os"
	"sync"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"gopkg.in/yaml.v3"
)

var (
	cfg           *cfgType.ServerConfig
	whitelistCfg  *cfgType.WhitelistConfig
	blacklistCfg  *cfgType.BlacklistConfig
	once          sync.Once
	whitelistOnce sync.Once
	blacklistOnce sync.Once
	mu            sync.Mutex
)

// GetConfig returns the server configuration.
func GetConfig() *cfgType.ServerConfig {
	return cfg
}

// GetWhitelistConfig returns the whitelist configuration.
func GetWhitelistConfig() *cfgType.WhitelistConfig {
	return whitelistCfg
}

// GetBlacklistConfig returns the blacklist configuration.
func GetBlacklistConfig() *cfgType.BlacklistConfig {
	return blacklistCfg
}

// ResetConfig clears the existing server configuration.
func ResetConfig() {
	mu.Lock()
	defer mu.Unlock()
	log.Config().Debug("Resetting server configuration")
	cfg = nil
	once = sync.Once{}
}

// ResetWhitelistConfig clears the existing whitelist configuration.
func ResetWhitelistConfig() {
	mu.Lock()
	defer mu.Unlock()
	log.Config().Debug("Resetting whitelist configuration")
	whitelistCfg = nil
	whitelistOnce = sync.Once{}
}

// ResetBlacklistConfig clears the existing blacklist configuration.
func ResetBlacklistConfig() {
	mu.Lock()
	defer mu.Unlock()
	log.Config().Debug("Resetting blacklist configuration")
	blacklistCfg = nil
	blacklistOnce = sync.Once{}
}

// LoadConfig loads the server configuration from config.yml.
func LoadConfig(filename string) (*cfgType.ServerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config cfgType.ServerConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	once.Do(func() {
		cfg = &config
		log.Config().Info("Server configuration loaded", "file", filename)
	})

	return cfg, nil
}

// LoadWhitelistConfig loads the whitelist configuration from whitelist.yml.
func LoadWhitelistConfig(filename string) (*cfgType.WhitelistConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config cfgType.WhitelistConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	whitelistOnce.Do(func() {
		whitelistCfg = &config
		log.Config().Info("Whitelist configuration loaded", "file", filename)
	})

	return whitelistCfg, nil
}

// LoadBlacklistConfig loads the blacklist configuration from blacklist.yml.
func LoadBlacklistConfig(filename string) (*cfgType.BlacklistConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config cfgType.BlacklistConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	blacklistOnce.Do(func() {
		blacklistCfg = &config
		log.Config().Info("Blacklist configuration loaded", "file", filename)
	})

	return blacklistCfg, nil
}
