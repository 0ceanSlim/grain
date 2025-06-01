package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// JSONLogWriter writes logs in a pretty-printed JSON array format
type JSONLogWriter struct {
	filePath    string
	level       slog.Level
	attrs       []slog.Attr
	mu          sync.Mutex // Mutex to protect file access
	maxSizeMB   int        // Maximum file size in MB
	backupCount int        // Number of backup files to keep
	lastCheck   time.Time  // Last time we checked the file size
	suppressedComponents map[string]bool
}

// NewJSONLogWriter creates a new instance of JSONLogWriter
func NewJSONLogWriter(filePath string, level slog.Level, maxSizeMB int, backupCount int, suppressedComponents map[string]bool) *JSONLogWriter {
	// Ensure the file exists with a valid JSON array
	ensureValidJSONFile(filePath)

	return &JSONLogWriter{
		filePath:    filePath,
		level:       level,
		maxSizeMB:   maxSizeMB,
		backupCount: backupCount,
		lastCheck:   time.Now(),
		suppressedComponents: suppressedComponents,
	}
}

// Handle writes logs in JSON format
func (j *JSONLogWriter) Handle(ctx context.Context, r slog.Record) error {
	// Check if log level is enabled
	if !j.Enabled(ctx, r.Level) {
		return nil
	}

	// Extract component from attributes to check suppression
	var component string
	for _, attr := range j.attrs {
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
	if shouldSuppressLog(component, r.Level, j.suppressedComponents) {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	// Check and manage log file size if needed - only check every 10 minutes
	if time.Since(j.lastCheck) > 10*time.Minute {
		j.checkAndManageJSONLogFile()
		j.lastCheck = time.Now()
	}

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

// checkAndTrimLogFile checks if log file exceeds max size and trims it if needed
func (j *JSONLogWriter) checkAndManageJSONLogFile() {
	if j.maxSizeMB <= 0 {
		return // No size limit
	}

	needsManagement, sizeBytes := checkLogSize(j.filePath, j.maxSizeMB)
	if !needsManagement {
		return
	}

	// Handle based on backup configuration
	if j.backupCount > 1 {
		// Rotate JSON log files
		Log().Info("JSON log file size exceeded limit, rotating...", 
			"file", j.filePath,
			"current_size_mb", float64(sizeBytes)/(1024*1024),
			"max_size_mb", j.maxSizeMB,
			"backup_count", j.backupCount)
		
		// Perform rotation for JSON files
		rotateLogFiles(j.filePath, j.backupCount)
		
		// Create a new empty JSON file
		ensureValidJSONFile(j.filePath)
	} else {
		// Trim JSON log file
		Log().Info("JSON log file size exceeded limit, trimming...", 
			"file", j.filePath,
			"current_size_mb", float64(sizeBytes)/(1024*1024),
			"max_size_mb", j.maxSizeMB)
		
		// For JSON files, we need special handling to keep the JSON structure valid
		trimJSONLogFile(j.filePath)
	}
}

func trimJSONLogFile(filePath string) error {
	// Read the current JSON array
	file, err := os.ReadFile(filePath)
	if err != nil {
		Log().Error("Error reading JSON log file for trimming", "file", filePath, "error", err)
		return err
	}

	var logs []json.RawMessage
	if err := json.Unmarshal(file, &logs); err != nil {
		Log().Error("Error parsing JSON logs for trimming", "file", filePath, "error", err)
		return err
	}

	// Calculate target size (20% of max)
	targetSizeRatio := 0.2 
	
	// Calculate how many logs to keep - prevent integer truncation
	logsToKeep := int(float64(len(logs)) * targetSizeRatio)
	if logsToKeep >= len(logs) || logsToKeep <= 0 {
		// Nothing to trim or would trim everything
		return nil
	}
	
	// Keep only the newer logs (the last logsToKeep entries)
	trimmedLogs := logs[len(logs)-logsToKeep:]

	// Write back to file
	newContent, err := json.MarshalIndent(trimmedLogs, "", "  ")
	if err != nil {
		Log().Error("Error creating trimmed JSON logs", "file", filePath, "error", err)
		return err
	}

	if err := os.WriteFile(filePath, newContent, 0644); err != nil {
		Log().Error("Error writing trimmed JSON file", "file", filePath, "error", err)
		return err
	}

	Log().Info("JSON log file trimmed successfully", 
		"file", filePath,
		"original_entries", len(logs),
		"remaining_entries", logsToKeep)
	
	return nil
}

// Required interface methods for slog.Handler
func (j *JSONLogWriter) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= j.level
}

func (j *JSONLogWriter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &JSONLogWriter{
		filePath:    j.filePath,
		level:       j.level,
		attrs:       append(j.attrs, attrs...),
		maxSizeMB:   j.maxSizeMB,
		backupCount: j.backupCount,
		lastCheck:   j.lastCheck,
		suppressedComponents: j.suppressedComponents,
	}
}

func (j *JSONLogWriter) WithGroup(name string) slog.Handler {
	return j // Grouping not supported in this implementation
}

// Close is now a no-op since logs are written properly on each entry.
func (j *JSONLogWriter) Close() {
	// No action needed, since the file is managed per log entry.
}
