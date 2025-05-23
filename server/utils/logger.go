package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	cfgTypes "github.com/0ceanslim/grain/config/types"
)

// Set the logging component for general utility functions
func utilLog() *slog.Logger {
	return GetLogger("util")
}

// LoggerRegistry maintains a map of all loggers by component name
type LoggerRegistry struct {
	main      *slog.Logger
	loggers   map[string]*slog.Logger
	handler   slog.Handler
	mu        sync.RWMutex
}

// Global registry instance
var Registry = &LoggerRegistry{
	loggers: make(map[string]*slog.Logger),
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
		handler = NewJSONLogWriter(logFilePath, logLevel, cfg.Logging.MaxSizeMB, cfg.Logging.BackupCount)
	} else {
		logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
		if err != nil {
			fmt.Printf("Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		handler = &PrettyLogWriter{
			output: logFile,
			level:  logLevel,
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

// Helper function to get basename
func basename(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// The GetLogger function now simply delegates to the registry
func GetLogger(component string) *slog.Logger {
	return Registry.Get(component)
}

// Additional global variable to track last trim time
var trimMutex        sync.Mutex

// StartPeriodicLogTrimmer starts a goroutine that periodically checks and manages log size
func StartPeriodicLogTrimmer(logFilePath string, maxSizeMB int, checkIntervalMinutes int, backupCount int) {
	go func() {
		ticker := time.NewTicker(time.Duration(checkIntervalMinutes) * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			err := manageLogFileSize(logFilePath, maxSizeMB, backupCount)
			if err != nil {
				utilLog().Error("Failed to manage log file size", 
					"file", logFilePath,
					"error", err)
			}
		}
	}()
	
	utilLog().Info("Started periodic log manager", 
		"check_interval_minutes", checkIntervalMinutes,
		"max_size_mb", maxSizeMB,
		"backup_count", backupCount)
}

// Handle log file rotation or trimming based on configuration
func manageLogFileSize(logFilePath string, maxSizeMB int, backupCount int) error {
	trimMutex.Lock()
	defer trimMutex.Unlock()

	needsManagement, sizeBytes := checkLogSize(logFilePath, maxSizeMB)
	if !needsManagement {
		return nil
	}

	// Log the action we're about to take
	if backupCount > 1 {
		utilLog().Info("Log file size exceeded limit, rotating logs...",
			"file", logFilePath,
			"current_size_mb", float64(sizeBytes)/(1024*1024),
			"max_size_mb", maxSizeMB,
			"backup_count", backupCount)
		return rotateLogFiles(logFilePath, backupCount)
	} else {
		utilLog().Info("Log file size exceeded limit, trimming...",
			"file", logFilePath,
			"current_size_mb", float64(sizeBytes)/(1024*1024),
			"max_size_mb", maxSizeMB,
			"target_size_mb", float64(maxSizeMB)*0.2)
		return trimLogFile(logFilePath, maxSizeMB)
	}
}

// rotateLogFiles handles the rotation of log files
func rotateLogFiles(logFilePath string, backupCount int) error {
	// For each backup slot, shift logs down
	for i := backupCount - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.bak%d", logFilePath, i-1)
		newPath := fmt.Sprintf("%s.bak%d", logFilePath, i)
		
		// Check if the source file exists before trying to rename it
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue // Skip if source doesn't exist
		}
		
		// Remove the destination file if it exists
		_ = os.Remove(newPath)
		
		// Rename the source to destination
		if err := os.Rename(oldPath, newPath); err != nil {
			utilLog().Error("Failed to rotate log file", 
				"from", oldPath, 
				"to", newPath, 
				"error", err)
			// Continue even if one rotation fails
		} else {
			utilLog().Debug("Rotated log file", 
				"from", oldPath, 
				"to", newPath)
		}
	}
	
	// Move the current log to .bak1
	backupPath := fmt.Sprintf("%s.bak1", logFilePath)
	
	// Try to read the current log
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		utilLog().Error("Failed to read current log file", 
			"file", logFilePath, 
			"error", err)
		return err
	}
	
	// Write it to backup
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		utilLog().Error("Failed to create backup log file", 
			"file", backupPath, 
			"error", err)
		return err
	}
	
	// Truncate current log file
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		utilLog().Error("Failed to truncate log file", 
			"file", logFilePath, 
			"error", err)
		return err
	}
	defer file.Close()
	
	utilLog().Info("Successfully rotated log files", 
		"main_log", logFilePath, 
		"backup_count", backupCount)
	
	return nil
}

// checkLogSize returns true if trimming is needed and the current file size
func checkLogSize(filePath string, maxSizeMB int) (bool, int64) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		utilLog().Error("Error checking log file size", "file", filePath, "error", err)
		return false, 0
	}
	
	maxSizeBytes := int64(maxSizeMB * 1024 * 1024)
	return fileInfo.Size() > maxSizeBytes, fileInfo.Size()
}

// trimLogFile to handle percentage-based trimming
func trimLogFile(filePath string, maxSizeMB int) error {
	// Read all lines
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}

	// Calculate target size (20% of max) - use float for the multiplication
	targetSize := int64(float64(maxSizeMB) * 0.2 * 1024.0 * 1024.0)
	
	// Calculate current size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	currentSize := fileInfo.Size()
	
	// Calculate how many lines to keep based on target size
	// Estimate average line size
	var avgLineSize int64 = 1 // Default to 1 to avoid division by zero
	if len(lines) > 0 {
		avgLineSize = currentSize / int64(len(lines))
		if avgLineSize == 0 {
			avgLineSize = 1 // Avoid division by zero
		}
	}
	
	// Calculate how many lines to keep
	linesToKeep := int(targetSize / avgLineSize)
	if linesToKeep >= len(lines) {
		// If we'd keep all lines anyway, just return
		return nil
	}
	
	// Keep the most recent lines
	remainingLines := lines[len(lines)-linesToKeep:]

	// Reopen file in write mode and overwrite with trimmed logs
	return os.WriteFile(filePath, []byte(strings.Join(remainingLines, "\n")+"\n"), 0644)
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
	filePath    string
	level       slog.Level
	attrs       []slog.Attr
	mu          sync.Mutex // Mutex to protect file access
	maxSizeMB   int        // Maximum file size in MB
	backupCount int        // Number of backup files to keep
	lastCheck   time.Time  // Last time we checked the file size
}

// NewJSONLogWriter creates a new instance of JSONLogWriter
func NewJSONLogWriter(filePath string, level slog.Level, maxSizeMB int, backupCount int) *JSONLogWriter {
	// Ensure the file exists with a valid JSON array
	ensureValidJSONFile(filePath)

	return &JSONLogWriter{
		filePath:    filePath,
		level:       level,
		maxSizeMB:   maxSizeMB,
		backupCount: backupCount,
		lastCheck:   time.Now(),
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
		utilLog().Info("JSON log file size exceeded limit, rotating...", 
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
		utilLog().Info("JSON log file size exceeded limit, trimming...", 
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
		utilLog().Error("Error reading JSON log file for trimming", "file", filePath, "error", err)
		return err
	}

	var logs []json.RawMessage
	if err := json.Unmarshal(file, &logs); err != nil {
		utilLog().Error("Error parsing JSON logs for trimming", "file", filePath, "error", err)
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
		utilLog().Error("Error creating trimmed JSON logs", "file", filePath, "error", err)
		return err
	}

	if err := os.WriteFile(filePath, newContent, 0644); err != nil {
		utilLog().Error("Error writing trimmed JSON file", "file", filePath, "error", err)
		return err
	}

	utilLog().Info("JSON log file trimmed successfully", 
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
	}
}

func (j *JSONLogWriter) WithGroup(name string) slog.Handler {
	return j // Grouping not supported in this implementation
}

// Close is now a no-op since logs are written properly on each entry.
func (j *JSONLogWriter) Close() {
	// No action needed, since the file is managed per log entry.
}
