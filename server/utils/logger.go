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
var Log *slog.Logger

// InitializeLogger loads config and sets up global logger
func InitializeLogger(configPath string) {
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

	// Convert log level from config
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

	// Create a LogWriter that writes ONLY to the file
	fileHandler := &LogWriter{output: logFile, level: logLevel}

	// Set global logger
	Log = slog.New(fileHandler)
}

// GetLogger returns a logger with a specific component field
func GetLogger(component string) *slog.Logger {
	if Log == nil {
		fmt.Println("Logger is not initialized. Returning default logger.")
		return slog.New(slog.NewTextHandler(os.Stdout, nil)) // Prevents crash
	}
	return Log.With("component", component)
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

// LogWriter writes logs ONLY to a file
type LogWriter struct {
	output *os.File
	level  slog.Level
	attrs  []slog.Attr
}

// Handle writes logs to the log file
func (h *LogWriter) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	// Format timestamp
	b.WriteString(r.Time.Format(time.RFC3339))
	b.WriteString(" ")

	// Format log level as [LEVEL]
	b.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(r.Level.String())))

	// Extract component from attributes (from stored attrs & record attrs)
	var component string

	// First, check stored attributes in LogWriter
	for _, attr := range h.attrs {
		if attr.Key == "component" {
			component = fmt.Sprintf("[%s] ", attr.Value.String())
			break
		}
	}

	// If not found, check the log record attributes
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "component" {
			component = fmt.Sprintf("[%s] ", attr.Value.String())
			return false
		}
		return true
	})

	// Append the component if found
	if component != "" {
		b.WriteString(component)
	}

	// Append message
	b.WriteString(r.Message)

	// Append remaining attributes (except component)
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "component" {
			b.WriteString(fmt.Sprintf(" %s=%v", attr.Key, attr.Value))
		}
		return true
	})

	// Write only to file (❌ No console output)
	if h.output != nil {
		_, err := fmt.Fprintln(h.output, b.String()) // Write to file
		if err != nil {
			return err
		}
		h.output.Sync() // ✅ Force immediate write to disk
	}

	return nil
}

// Required interface methods for slog.Handler
func (h *LogWriter) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}
func (h *LogWriter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogWriter{
		output: h.output,
		level:  h.level,
		attrs:  append(h.attrs, attrs...),
	}
}
func (h *LogWriter) WithGroup(name string) slog.Handler {
	return h
}
