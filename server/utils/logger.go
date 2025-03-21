package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Logging LogConfig `yaml:"logging"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// Logger instance
var Logger *slog.Logger

// ANSI color codes for terminal output
var levelColors = map[slog.Level]string{
	slog.LevelDebug: "\033[36m", // Cyan
	slog.LevelInfo:  "\033[32m", // Green
	slog.LevelWarn:  "\033[33m", // Yellow
	slog.LevelError: "\033[31m", // Red
}

// Reset color (outside of the map)
const colorReset = "\033[0m"

// MultiHandler sends logs to multiple handlers
type MultiHandler struct {
	handlers []slog.Handler
}

// Enabled checks if the given level is enabled for any handler
func (m MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle sends logs to all registered handlers
func (m MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		h.Handle(ctx, record)
	}
	return nil
}

// WithAttrs returns a new MultiHandler with additional attributes
func (m MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return MultiHandler{handlers: newHandlers}
}

// WithGroup returns a new MultiHandler with grouped attributes
func (m MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return MultiHandler{handlers: newHandlers}
}

func InitializeLogger(configPath string) {
	// Load YAML config
	cfg := Config{}
	file, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	// Debug: Print loaded config
	fmt.Printf("Loaded config: %+v\n", cfg)

	// Trim spaces and convert level to lowercase
	cfg.Logging.Level = strings.TrimSpace(strings.ToLower(cfg.Logging.Level))

	// Convert log level using slog's built-in function
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		fmt.Printf("Invalid log level in config: %s\n", cfg.Logging.Level)
		os.Exit(1)
	}

	// Open log file
	logFile, err := os.OpenFile(cfg.Logging.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	// Define handlers for file and console output
	fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: logLevel})
	consoleHandler := NewColorConsoleHandler(os.Stdout, logLevel)

	// Use MultiHandler to log to both file and console
	Logger = slog.New(MultiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}})
}

// GetLogger returns a logger with a specific component field
func GetLogger(component string) *slog.Logger {
	return Logger.With("component", component)
}

// NewColorConsoleHandler returns a handler that adds color coding to console logs
func NewColorConsoleHandler(output *os.File, level slog.Level) slog.Handler {
	return slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Colorize the level field
			if a.Key == slog.LevelKey {
				level, _ := a.Value.Any().(slog.Level) // Ensure type safety
				if color, exists := levelColors[level]; exists {
					a.Value = slog.StringValue(color + strings.ToUpper(a.Value.String()) + colorReset)
				}
			}
			return a
		},
	})
}
