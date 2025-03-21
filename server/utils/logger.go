package utils

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Logging LogConfig `yaml:"logging"`
}

type LogConfig struct {
	Level     string `yaml:"level"`
	File      string `yaml:"file"`
	MaxSizeMB int    `yaml:"max_log_size_mb"`
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

	// Convert log level
	cfg.Logging.Level = strings.TrimSpace(strings.ToLower(cfg.Logging.Level))
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

	// Define handlers
	fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: logLevel})
	consoleHandler := NewColorConsoleHandler(os.Stdout, logLevel)
	Logger = slog.New(MultiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}})

	// Start log trimming in a separate goroutine
	go func() {
		for {
			TrimLogFile(cfg.Logging.File, cfg.Logging.MaxSizeMB)
			time.Sleep(10 * time.Second) // Check every 10 seconds
		}
	}()
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

// TrimLogFile checks log size and trims the oldest 20% if needed
func TrimLogFile(filePath string, maxSizeMB int) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error checking log file: %v\n", err)
		return
	}

	// Convert max size to bytes
	maxSizeBytes := maxSizeMB * 1024 * 1024

	// If file size exceeds the limit, start trimming
	if fileInfo.Size() > int64(maxSizeBytes) {
		fmt.Println("Log file size exceeded limit, trimming...")

		// Read all lines
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("Error opening log file: %v\n", err)
			return
		}
		defer file.Close()

		var lines []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		// Calculate how many lines to keep (80%)
		trimCount := len(lines) / 5 // 20% to remove
		remainingLines := lines[trimCount:]

		// Reopen file in write mode and overwrite it with trimmed logs
		err = os.WriteFile(filePath, []byte(strings.Join(remainingLines, "\n")+"\n"), 0644)
		if err != nil {
			fmt.Printf("Error writing trimmed log file: %v\n", err)
		} else {
			fmt.Println("Log file trimmed successfully")
		}
	}
}
