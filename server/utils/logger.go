package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
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
	Structure bool   `yaml:"structure"`
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

	// Determine log file name based on structure setting
	var logFilePath string
	if cfg.Logging.Structure {
		logFilePath = cfg.Logging.File + ".json"
	} else {
		logFilePath = cfg.Logging.File + ".log"
	}

	// Choose between structured JSON logs and pretty logs
	var handler slog.Handler
	if cfg.Logging.Structure {
		handler = NewJSONLogWriter(logFilePath, logLevel, cfg.Logging.MaxSizeMB)
	} else {
		logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
		if err != nil {
			fmt.Printf("Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		handler = &PrettyLogWriter{output: logFile, level: logLevel}
	}

	// Set global logger
	Log = slog.New(handler)
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

// PrettyLogWriter writes logs ONLY to a file
type PrettyLogWriter struct {
	output *os.File
	level  slog.Level
	attrs  []slog.Attr
}

// Handle writes logs to the log file
func (h *PrettyLogWriter) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	// Format timestamp
	b.WriteString(r.Time.Format(time.RFC3339))
	b.WriteString(" ")

	// Format log level as [LEVEL]
	b.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(r.Level.String())))

	// Extract component from attributes
	var component string
	for _, attr := range h.attrs {
		if attr.Key == "component" {
			component = fmt.Sprintf("[%s] ", attr.Value.String())
			break
		}
	}
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

	// Write to file ( No console output)
	if h.output != nil {
		_, err := fmt.Fprintln(h.output, b.String()) // Write to file
		if err != nil {
			return err
		}
		h.output.Sync() // immediate write to disk
	}

	return nil
}

// Required interface methods for slog.Handler
func (h *PrettyLogWriter) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}
func (h *PrettyLogWriter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &PrettyLogWriter{
		output: h.output,
		level:  h.level,
		attrs:  append(h.attrs, attrs...),
	}
}
func (h *PrettyLogWriter) WithGroup(name string) slog.Handler {
	return h
}

// JSONLogWriter writes logs in a pretty-printed JSON array format
type JSONLogWriter struct {
	filePath  string
	level     slog.Level
	attrs     []slog.Attr
	mu        sync.Mutex // Mutex to protect file access
	maxSizeMB int        // Maximum file size in MB
}

// NewJSONLogWriter creates a new instance of JSONLogWriter
func NewJSONLogWriter(filePath string, level slog.Level, maxSizeMB int) *JSONLogWriter {
	// Ensure the file exists with a valid JSON array
	ensureValidJSONFile(filePath)

	return &JSONLogWriter{
		filePath:  filePath,
		level:     level,
		maxSizeMB: maxSizeMB,
	}
}

// ensureValidJSONFile makes sure the file exists and contains a valid JSON array
func ensureValidJSONFile(filePath string) {
	// Check if file exists
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// Create new file with empty array
		if err := os.WriteFile(filePath, []byte("[\n]\n"), 0644); err != nil {
			fmt.Printf("Error creating JSON log file: %v\n", err)
		}
		return
	}

	// File exists, check if it's a valid JSON array
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading JSON log file: %v\n", err)
		return
	}

	// Trim whitespace
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		// Empty file, write empty array
		if err := os.WriteFile(filePath, []byte("[\n]\n"), 0644); err != nil {
			fmt.Printf("Error writing empty JSON array: %v\n", err)
		}
		return
	}

	// Try to parse as JSON array
	var logs []json.RawMessage
	if err := json.Unmarshal(trimmed, &logs); err != nil {
		fmt.Printf("JSON log file is not a valid array, resetting: %v\n", err)
		if err := os.WriteFile(filePath, []byte("[\n]\n"), 0644); err != nil {
			fmt.Printf("Error resetting JSON log file: %v\n", err)
		}
	}
}

// Handle writes logs in JSON format
func (j *JSONLogWriter) Handle(ctx context.Context, r slog.Record) error {
	// Check if log level is enabled
	if !j.Enabled(ctx, r.Level) {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	// Check and trim log file if needed
	j.checkAndTrimLogFile()

	// Collect all attributes including those from WithAttrs
	allAttrs := make(map[string]interface{})

	// First add attrs from WithAttrs
	for _, attr := range j.attrs {
		allAttrs[attr.Key] = attr.Value.String()
	}

	// Then add attrs from the record
	r.Attrs(func(attr slog.Attr) bool {
		allAttrs[attr.Key] = attr.Value.String()
		return true
	})

	// Create log entry
	logEntry := map[string]interface{}{
		"time":  r.Time.Format(time.RFC3339),
		"level": r.Level.String(),
		"msg":   r.Message,
	}

	// Add component if it exists
	if component, ok := allAttrs["component"]; ok {
		logEntry["component"] = component
		delete(allAttrs, "component") // Remove it to avoid duplication
	}

	// Add remaining attributes
	for k, v := range allAttrs {
		logEntry[k] = v
	}

	// Convert to pretty JSON
	jsonData, err := json.MarshalIndent(logEntry, "  ", "  ")
	if err != nil {
		return err
	}

	// Read existing file content
	file, err := os.ReadFile(j.filePath)
	if err != nil {
		return err
	}

	// Find the closing bracket
	idx := bytes.LastIndex(file, []byte("]"))
	if idx == -1 {
		// If not found, something is wrong with the file, recreate it
		newContent := fmt.Sprintf("[\n  %s\n]\n", jsonData)
		return os.WriteFile(j.filePath, []byte(newContent), 0644)
	}

	// Prepare new content
	var newContent []byte
	if idx <= 2 { // Empty array
		newContent = append(file[:idx], []byte(fmt.Sprintf("  %s\n]", jsonData))...)
	} else {
		newContent = append(file[:idx], []byte(fmt.Sprintf(",\n  %s\n]", jsonData))...)
	}

	// Write back to file
	return os.WriteFile(j.filePath, newContent, 0644)
}

// checkAndTrimLogFile checks if log file exceeds max size and trims it if needed
func (j *JSONLogWriter) checkAndTrimLogFile() {
	if j.maxSizeMB <= 0 {
		return // No size limit
	}

	fileInfo, err := os.Stat(j.filePath)
	if err != nil {
		fmt.Printf("Error checking JSON log file size: %v\n", err)
		return
	}

	// Convert max size to bytes
	maxSizeBytes := j.maxSizeMB * 1024 * 1024

	// If file size exceeds the limit, start trimming
	if fileInfo.Size() > int64(maxSizeBytes) {
		fmt.Println("JSON log file size exceeded limit, trimming...")

		// Read the current JSON array
		file, err := os.ReadFile(j.filePath)
		if err != nil {
			fmt.Printf("Error reading JSON log file: %v\n", err)
			return
		}

		var logs []json.RawMessage
		if err := json.Unmarshal(file, &logs); err != nil {
			fmt.Printf("Error parsing JSON logs: %v\n", err)
			return
		}

		// Calculate how many logs to keep (80%)
		trimCount := len(logs) / 5 // 20% to remove
		if trimCount <= 0 {
			return // Not enough logs to trim
		}

		// Keep only the newer logs
		remainingLogs := logs[trimCount:]

		// Write back to file
		newContent, err := json.MarshalIndent(remainingLogs, "", "  ")
		if err != nil {
			fmt.Printf("Error creating trimmed JSON logs: %v\n", err)
			return
		}

		if err := os.WriteFile(j.filePath, newContent, 0644); err != nil {
			fmt.Printf("Error writing trimmed JSON file: %v\n", err)
		} else {
			fmt.Println("JSON log file trimmed successfully")
		}
	}
}

// Required interface methods for slog.Handler
func (j *JSONLogWriter) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= j.level
}

func (j *JSONLogWriter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &JSONLogWriter{
		filePath:  j.filePath,
		level:     j.level,
		attrs:     append(j.attrs, attrs...),
		maxSizeMB: j.maxSizeMB,
	}
}

func (j *JSONLogWriter) WithGroup(name string) slog.Handler {
	return j // Grouping not supported in this implementation
}

// Close is now a no-op since logs are written properly on each entry.
func (j *JSONLogWriter) Close() {
	// No action needed, since the file is managed per log entry.
}
