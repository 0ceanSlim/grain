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

	// Define handlers using `SimpleLogHandler`
	fileHandler := &SimpleLogHandler{output: logFile, level: logLevel}
	consoleHandler := &SimpleLogHandler{output: nil, level: logLevel} // No file output for console

	// Set global Log variable
	Log = slog.New(MultiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}})
}

type CustomErrorHandler struct {
	Handler slog.Handler
}

// Handle modifies error messages before passing to the base handler
func (h CustomErrorHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelError {
		// Iterate over attributes to find an error
		r.Attrs(func(attr slog.Attr) bool {
			if err, ok := attr.Value.Any().(error); ok {
				r.Message = err.Error() // Set error message
			}
			return true // Continue iteration
		})
	}
	return h.Handler.Handle(ctx, r)
}

// Other required methods for Handler interface
func (h CustomErrorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}
func (h CustomErrorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return CustomErrorHandler{Handler: h.Handler.WithAttrs(attrs)}
}
func (h CustomErrorHandler) WithGroup(name string) slog.Handler {
	return CustomErrorHandler{Handler: h.Handler.WithGroup(name)}
}

// GetLogger returns a logger with a specific component field
func GetLogger(component string) *slog.Logger {
	return Log.With("component", fmt.Sprintf("[%s]", component))
}

// NewColorConsoleHandler returns a handler that adds color coding to console logs
func NewColorConsoleHandler(output *os.File, level slog.Level) slog.Handler {
	return slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey: // Format timestamp without key
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
			case slog.LevelKey: // Format level as [LEVEL]
				a.Value = slog.StringValue(fmt.Sprintf("[%s]", strings.ToUpper(a.Value.String())))
			case slog.MessageKey: // Keep message clean (remove "msg=" prefix)
				return a // No modification needed
			default:
				return slog.Attr{} // Remove all other attributes (component, etc.)
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

type LogFormatterHandler struct {
	Handler slog.Handler
}

func (h LogFormatterHandler) Handle(ctx context.Context, r slog.Record) error {
	// Manually format log entry
	var b strings.Builder

	// Format timestamp
	b.WriteString(r.Time.Format(time.RFC3339))
	b.WriteString(" ")

	// Format log level as [LEVEL]
	b.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(r.Level.String())))

	// Append message
	b.WriteString(r.Message)

	// Handle extra attributes (optional)
	r.Attrs(func(attr slog.Attr) bool {
		b.WriteString(fmt.Sprintf(" %v", attr.Value))
		return true
	})

	// Write to both console and file
	fmt.Fprintln(os.Stdout, b.String()) // ✅ Print to console

	// Write to file using the actual handler
	r = slog.Record{
		Time:    r.Time,
		Level:   r.Level,
		Message: b.String(),
	}
	return h.Handler.Handle(ctx, r) // ✅ Ensure logs are sent to the file handler
}


// Required interface methods
func (h LogFormatterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}
func (h LogFormatterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return LogFormatterHandler{Handler: h.Handler.WithAttrs(attrs)}
}
func (h LogFormatterHandler) WithGroup(name string) slog.Handler {
	return LogFormatterHandler{Handler: h.Handler.WithGroup(name)}
}

type SimpleLogHandler struct {
	output *os.File
	level  slog.Level
}

func (h *SimpleLogHandler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	// Format timestamp
	b.WriteString(r.Time.Format(time.RFC3339))
	b.WriteString(" ")

	// Format log level as [LEVEL]
	b.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(r.Level.String())))

	// Check for component
	var component string
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "component" {
			component = attr.Value.String()
			return false // Stop iterating
		}
		return true
	})

	// Append component if it exists
	if component != "" {
		b.WriteString(component + " ")
	}

	// Append message
	b.WriteString(r.Message)

	// Handle extra attributes (optional)
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "component" { // Avoid duplicate component printing
			b.WriteString(fmt.Sprintf(" %v", attr.Value))
		}
		return true
	})

	// Write to console
	fmt.Println(b.String())

	// Write to file if enabled
	if h.output != nil {
		fmt.Fprintln(h.output, b.String())
	}

	return nil
}

// Required interface methods
func (h *SimpleLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *SimpleLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // No structured attributes needed
}

func (h *SimpleLogHandler) WithGroup(name string) slog.Handler {
	return h // No grouping needed
}
