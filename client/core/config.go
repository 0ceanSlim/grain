package core

import (
	"fmt"
	"strings"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// Config holds client-specific configuration
type Config struct {
	IndexRelays       []string      `json:"index_relays"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	MaxConnections    int           `json:"max_connections"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
	KeepAlive         bool          `json:"keep_alive"`
	UserAgent         string        `json:"user_agent"`

	// Outbox-pool lifecycle (#56). Zero values fall back to built-in defaults
	// in NewRelayPool / the lease methods, so older callers that build a Config
	// by hand keep working.
	DialConcurrency int           `json:"dial_concurrency"` // max simultaneous dials
	IdleTTL         time.Duration `json:"idle_ttl"`         // evict a 0-lease conn after this idle span
	BackoffBase     time.Duration `json:"backoff_base"`     // first dial-retry backoff
	BackoffMax      time.Duration `json:"backoff_max"`      // dial-retry backoff ceiling
}

// DefaultConfig returns a sensible default configuration. The IndexRelays
// seed list mirrors the indexer-relay role described in #56: relays that
// host metadata and relay lists for everyone, used to resolve NIP-65 /
// DM-relay lists for arbitrary users.
func DefaultConfig() *Config {
	return &Config{
		IndexRelays: []string{
			"wss://profiles.nostr1.com",
			"wss://directory.yabu.me",
			"wss://user.kindpag.es",
			"wss://indexer.coracle.social",
			"wss://purplepag.es",
		},
		ConnectionTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxConnections:    256, // soft cap; the outbox pool holds many users' relays
		RetryAttempts:     3,
		RetryDelay:        2 * time.Second,
		KeepAlive:         true,
		UserAgent:         "grain-client/1.0",
		DialConcurrency:   16,
		IdleTTL:           120 * time.Second,
		BackoffBase:       2 * time.Second,
		BackoffMax:        60 * time.Second,
	}
}

// ConfigFromServerConfig creates a client config from server configuration
func ConfigFromServerConfig(serverCfg *cfgType.ServerConfig) *Config {
	// Start with defaults
	config := DefaultConfig()

	// Override with values from YAML if provided
	if serverCfg != nil && len(serverCfg.Client.IndexRelays) > 0 {
		config.IndexRelays = serverCfg.Client.IndexRelays
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

	if len(c.IndexRelays) == 0 {
		return fmt.Errorf("at least one index relay must be specified")
	}

	// Validate relay URLs (basic check)
	for _, relay := range c.IndexRelays {
		if len(relay) == 0 {
			return fmt.Errorf("empty relay URL found")
		}

		// Check for valid WebSocket URL schemes
		if !strings.HasPrefix(relay, "ws://") && !strings.HasPrefix(relay, "wss://") {
			return fmt.Errorf("invalid relay URL format: %s", relay)
		}

		// Basic length validation (minimum viable WebSocket URL)
		if len(relay) < 8 { // Minimum: "ws://x.x" = 8 chars
			return fmt.Errorf("relay URL too short: %s", relay)
		}
	}

	return nil
}
