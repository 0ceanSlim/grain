package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	cfgTypes "github.com/0ceanslim/grain/config/types"
)

// LoggerRegistry maintains a map of all loggers by component name
type LoggerRegistry struct {
	main      *slog.Logger
	loggers   map[string]*slog.Logger
	handler   slog.Handler
	suppressedComponents map[string]bool
	mu        sync.RWMutex
}

// Global registry instance
var Registry = &LoggerRegistry{
	loggers: make(map[string]*slog.Logger),
	suppressedComponents: make(map[string]bool),
}


// Get returns a logger for the specified component
func (r *LoggerRegistry) Get(component string) *slog.Logger {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If the registry hasn't been initialized yet, return a no-op logger
	if r.main == nil {
		noopHandler := slog.NewTextHandler(io.Discard, nil)
		return slog.New(noopHandler).With("component", component)
	}

	// Check if we have a pre-created logger for this component
	if logger, exists := r.loggers[component]; exists {
		return logger
	}

	// If not found, create one on-demand (shouldn't happen with proper initialization)
	return r.main.With("component", component)
}



// The GetLogger function now simply delegates to the registry
func GetLogger(component string) *slog.Logger {
	return Registry.Get(component)
}

// InitializeLoggers sets up the central logging system with the given configuration
func InitializeLoggers(cfg *cfgTypes.ServerConfig) {
	// Convert log level from config
	cfg.Logging.Level = strings.TrimSpace(strings.ToLower(cfg.Logging.Level))
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		fmt.Printf("Invalid log level in config: %s\n", cfg.Logging.Level)
		os.Exit(1)
	}

	// Build suppressed components map for fast lookup
	suppressedComponents := make(map[string]bool)
	for _, component := range cfg.Logging.SuppressComponents {
		suppressedComponents[strings.TrimSpace(component)] = true
	}

	// Determine log file name based on structure setting
	var logFilePath string
	if cfg.Logging.Structure {
		logFilePath = cfg.Logging.File + ".json"
	} else {
		logFilePath = cfg.Logging.File + ".log"
	}

	// Ensure directory exists
	dir := strings.TrimSuffix(cfg.Logging.File, basename(cfg.Logging.File))
	if dir != "" && dir != cfg.Logging.File {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
		}
	}

    // Choose between structured JSON logs and pretty logs
	var handler slog.Handler
	if cfg.Logging.Structure {
		// Pass backup count to JSONLogWriter
		handler = NewJSONLogWriter(logFilePath, logLevel, cfg.Logging.MaxSizeMB, cfg.Logging.BackupCount, suppressedComponents)
	} else {
		logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
		if err != nil {
			fmt.Printf("Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		handler = &PrettyLogWriter{
			output: logFile,
			level:  logLevel,
			suppressedComponents: suppressedComponents,
		}
	}

    // Create main logger
    mainLogger := slog.New(handler)

    // Print a console message just for initialization confirmation. Supressed Line in produciton. 
    //fmt.Printf("Logger initialized: writing to %s\n", logFilePath)

    // Now create all the component loggers
    // Pre-creating all loggers you'll need in the application
    components := []string{
        "main", "mongo", "mongo-query", "mongo-store", "mongo-purge", "mongo-event",
        "event-handler", "req-handler", "auth-handler", "close-handler",
        "client", "config", "util", "event-validation", "conn-manager", "user-sync",
    }

    // Create a map of loggers before acquiring the lock
    tempLoggers := make(map[string]*slog.Logger, len(components))
    for _, component := range components {
        tempLoggers[component] = mainLogger.With("component", component)
    }

    // Lock the registry, update it, and unlock immediately
    Registry.mu.Lock()
    Registry.handler = handler
    Registry.main = mainLogger
    Registry.loggers = tempLoggers
	Registry.suppressedComponents = suppressedComponents
    Registry.mu.Unlock()

    // Now that we've released the lock, we can safely log
    GetLogger("main").Info("Logger system initialized",
        "level", cfg.Logging.Level,
        "file", logFilePath,
        "structured", cfg.Logging.Structure,
        "components", len(components))
		    
    // Start periodic log management
	checkInterval := 10 // Default to 10 minutes if not configured
	if cfg.Logging.CheckIntervalMin > 0 {
		checkInterval = cfg.Logging.CheckIntervalMin
	}

	// Default to 1 for backup_count if not specified (trim rather than rotate)
	backupCount := 1
	if cfg.Logging.BackupCount > 0 {
		backupCount = cfg.Logging.BackupCount
	}

	StartPeriodicLogTrimmer(logFilePath, cfg.Logging.MaxSizeMB, checkInterval, backupCount)
}

// shouldSuppressLog determines if a log should be suppressed based on component and level
func shouldSuppressLog(component string, level slog.Level, suppressedComponents map[string]bool) bool {
	if !suppressedComponents[component] {
		return false
	}
	// Only suppress INFO and DEBUG levels for suppressed components
	return level <= slog.LevelInfo
}