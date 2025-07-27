package core

import (
	"fmt"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// Config holds client-specific configuration
type Config struct {
	DefaultRelays     []string      `json:"default_relays"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	MaxConnections    int           `json:"max_connections"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
	KeepAlive         bool          `json:"keep_alive"`
	UserAgent         string        `json:"user_agent"`
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultRelays: []string{
			"wss://relay.damus.io",
			"wss://nos.lol",
			"wss://relay.nostr.band",
		},
		ConnectionTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxConnections:    10,
		RetryAttempts:     3,
		RetryDelay:        2 * time.Second,
		KeepAlive:         true,
		UserAgent:         "grain-client/1.0",
	}
}

// ConfigFromServerConfig creates a client config from server configuration
func ConfigFromServerConfig(serverCfg *cfgType.ServerConfig) *Config {
	// Start with defaults
	config := DefaultConfig()

	// Override with values from YAML if provided
	if serverCfg != nil && len(serverCfg.Client.DefaultRelays) > 0 {
		config.DefaultRelays = serverCfg.Client.DefaultRelays
	}

	if serverCfg != nil && serverCfg.Client.ConnectionTimeout > 0 {
		config.ConnectionTimeout = time.Duration(serverCfg.Client.ConnectionTimeout) * time.Second
	}

	if serverCfg != nil && serverCfg.Client.ReadTimeout > 0 {
		config.ReadTimeout = time.Duration(serverCfg.Client.ReadTimeout) * time.Second
	}

	if serverCfg != nil && serverCfg.Client.WriteTimeout > 0 {
		config.WriteTimeout = time.Duration(serverCfg.Client.WriteTimeout) * time.Second
	}

	if serverCfg != nil && serverCfg.Client.MaxConnections > 0 {
		config.MaxConnections = serverCfg.Client.MaxConnections
	}

	if serverCfg != nil && serverCfg.Client.RetryAttempts >= 0 {
		config.RetryAttempts = serverCfg.Client.RetryAttempts
	}

	if serverCfg != nil && serverCfg.Client.RetryDelay > 0 {
		config.RetryDelay = time.Duration(serverCfg.Client.RetryDelay) * time.Second
	}

	if serverCfg != nil {
		config.KeepAlive = serverCfg.Client.KeepAlive
	}

	if serverCfg != nil && serverCfg.Client.UserAgent != "" {
		config.UserAgent = serverCfg.Client.UserAgent
	}

	return config
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}

	if c.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}

	if c.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}

	if c.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}

	if c.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative")
	}

	if len(c.DefaultRelays) == 0 {
		return fmt.Errorf("at least one default relay must be specified")
	}

	// Validate relay URLs (basic check)
	for _, relay := range c.DefaultRelays {
		if len(relay) == 0 {
			return fmt.Errorf("empty relay URL found")
		}
		if len(relay) < 6 || (relay[:4] != "ws://" && relay[:5] != "wss://") {
			return fmt.Errorf("invalid relay URL format: %s", relay)
		}
	}

	return nil
}
