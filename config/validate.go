package config

import (
	"fmt"
	"strings"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// ValidateAndApplyDefaults checks the config for zero-valued fields and applies
// sensible defaults. It returns a list of warnings for each default applied and
// an error if the config is truly broken.
func ValidateAndApplyDefaults(cfg *cfgType.ServerConfig) (warnings []string, err error) {
	// Database defaults
	if cfg.Database.Path == "" {
		cfg.Database.Path = "data"
		warnings = append(warnings, "database.path was empty, defaulting to \"data\"")
	}
	if cfg.Database.MapSizeMB == 0 {
		cfg.Database.MapSizeMB = 4096
		warnings = append(warnings, "database.map_size_mb was 0, defaulting to 4096 (4 GB)")
	}

	// Server defaults
	if cfg.Server.Port == "" {
		cfg.Server.Port = ":8181"
		warnings = append(warnings, "server.port was empty, defaulting to \":8181\"")
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 10
		warnings = append(warnings, "server.read_timeout was 0, defaulting to 10")
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 10
		warnings = append(warnings, "server.write_timeout was 0, defaulting to 10")
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 300
		warnings = append(warnings, "server.idle_timeout was 0, defaulting to 300")
	}
	if cfg.Server.MaxConnections == 0 {
		cfg.Server.MaxConnections = 1000
		warnings = append(warnings, "server.max_connections was 0, defaulting to 1000")
	}
	if cfg.Server.MaxSubscriptionsPerClient == 0 {
		cfg.Server.MaxSubscriptionsPerClient = 10
		warnings = append(warnings, "server.max_subscriptions_per_client was 0, defaulting to 10")
	}
	if cfg.Server.ImplicitReqLimit == 0 {
		cfg.Server.ImplicitReqLimit = 500
		warnings = append(warnings, "server.implicit_req_limit was 0, defaulting to 500")
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
		warnings = append(warnings, "logging.level was empty, defaulting to \"info\"")
	}
	if cfg.Logging.File == "" {
		cfg.Logging.File = "debug.log"
		warnings = append(warnings, "logging.file was empty, defaulting to \"debug.log\"")
	}
	if cfg.Logging.MaxSizeMB == 0 {
		cfg.Logging.MaxSizeMB = 10
		warnings = append(warnings, "logging.max_log_size_mb was 0, defaulting to 10")
	}
	if cfg.Logging.BackupCount == 0 {
		cfg.Logging.BackupCount = 2
		warnings = append(warnings, "logging.backup_count was 0, defaulting to 2")
	}
	if cfg.Logging.CheckIntervalMin == 0 {
		cfg.Logging.CheckIntervalMin = 5
		warnings = append(warnings, "logging.check_interval_min was 0, defaulting to 5")
	}

	// Rate limit defaults
	if cfg.RateLimit.MaxEventSize == 0 {
		cfg.RateLimit.MaxEventSize = 524288
		warnings = append(warnings, "rate_limit.max_event_size was 0, defaulting to 524288 (512 KB)")
	}

	// Validation errors (after defaults are applied)
	if !strings.HasPrefix(cfg.Server.Port, ":") {
		err = fmt.Errorf("server.port %q is invalid: must start with \":\" (e.g. \":8181\")", cfg.Server.Port)
	}

	return warnings, err
}
