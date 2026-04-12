package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	dataDir       string
)

// SetDataDir sets the resolved data directory path for the application.
func SetDataDir(dir string) { dataDir = dir }

// GetDataDir returns the resolved data directory path.
func GetDataDir() string { return dataDir }

// ConfigPath returns the full path for a file within the data directory.
func ConfigPath(filename string) string {
	return filepath.Join(dataDir, filename)
}

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

// applyEnvironmentOverrides applies environment variable overrides to the config
func applyEnvironmentOverrides(config *cfgType.ServerConfig) {
	log.Config().Debug("Checking for environment variable overrides")

	// Database path override
	if dbPath := os.Getenv("NDB_PATH"); dbPath != "" {
		log.Config().Info("Overriding database path from environment variable",
			"original", config.Database.Path,
			"override", dbPath)
		config.Database.Path = dbPath
	}

	// Server port override
	if serverPort := os.Getenv("SERVER_PORT"); serverPort != "" {
		log.Config().Info("Overriding server port from environment variable",
			"original", config.Server.Port,
			"override", serverPort)
		config.Server.Port = ":" + serverPort
	}

	// Log level override
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		log.Config().Info("Overriding log level from environment variable",
			"original", config.Logging.Level,
			"override", logLevel)
		config.Logging.Level = logLevel
	}

	// Environment type override
	if grainEnv := os.Getenv("GRAIN_ENV"); grainEnv != "" {
		log.Config().Info("Environment set via GRAIN_ENV", "environment", grainEnv)
		// You can use this for environment-specific behavior if needed
	}
}

// Update your LoadConfig function to call this:
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

	// Detect outdated config format (e.g., old mongodb section)
	if err := CheckAndMigrateConfig(filename); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: config migration check failed: %v\n", err)
	}

	// Validate config and apply defaults for missing fields
	warnings, validationErr := ValidateAndApplyDefaults(&config)
	if len(warnings) > 0 {
		fmt.Fprintln(os.Stderr, "Config validation — defaults applied:")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
		fmt.Fprintln(os.Stderr, "")
	}
	if validationErr != nil {
		return nil, fmt.Errorf("config validation error: %w", validationErr)
	}

	// Apply environment variable overrides (after defaults, so env vars win)
	applyEnvironmentOverrides(&config)

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
