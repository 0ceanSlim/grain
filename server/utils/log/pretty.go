package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// PrettyLogWriter writes logs ONLY to a file
type PrettyLogWriter struct {
	output *os.File
	level  slog.Level
	attrs  []slog.Attr
	suppressedComponents map[string]bool
}

//func NewPrettyLogWriter(file *os.File, level slog.Level, suppressedComponents map[string]bool) *PrettyLogWriter {
//    // Constructor
//}

// Handle writes logs to the log file
func (h *PrettyLogWriter) Handle(ctx context.Context, r slog.Record) error {
	// Extract component from attributes first
	var component string
	for _, attr := range h.attrs {
		if attr.Key == "component" {
			component = attr.Value.String()
			break
		}
	}
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "component" {
			component = attr.Value.String()
			return false
		}
		return true
	})

	// Check if this log should be suppressed
	if shouldSuppressLog(component, r.Level, h.suppressedComponents) {
		return nil
	}

	var b strings.Builder

	// Format timestamp
	b.WriteString(r.Time.Format(time.RFC3339))
	b.WriteString(" ")

	// Format log level as [LEVEL]
	b.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(r.Level.String())))

	// Append the component if found
	if component != "" {
		b.WriteString(fmt.Sprintf("[%s] ", component))
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
		suppressedComponents: h.suppressedComponents,
	}
}

func (h *PrettyLogWriter) WithGroup(name string) slog.Handler {
	return h
}