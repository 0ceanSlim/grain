package log

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

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
				logLog().Error("Failed to manage log file size", 
					"file", logFilePath,
					"error", err)
			}
		}
	}()
	
	logLog().Info("Started periodic log manager", 
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
		logLog().Info("Log file size exceeded limit, rotating logs...",
			"file", logFilePath,
			"current_size_mb", float64(sizeBytes)/(1024*1024),
			"max_size_mb", maxSizeMB,
			"backup_count", backupCount)
		return rotateLogFiles(logFilePath, backupCount)
	} else {
		logLog().Info("Log file size exceeded limit, trimming...",
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
			logLog().Error("Failed to rotate log file", 
				"from", oldPath, 
				"to", newPath, 
				"error", err)
			// Continue even if one rotation fails
		} else {
			logLog().Debug("Rotated log file", 
				"from", oldPath, 
				"to", newPath)
		}
	}
	
	// Move the current log to .bak1
	backupPath := fmt.Sprintf("%s.bak1", logFilePath)
	
	// Try to read the current log
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		logLog().Error("Failed to read current log file", 
			"file", logFilePath, 
			"error", err)
		return err
	}
	
	// Write it to backup
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		logLog().Error("Failed to create backup log file", 
			"file", backupPath, 
			"error", err)
		return err
	}
	
	// Truncate current log file
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		logLog().Error("Failed to truncate log file", 
			"file", logFilePath, 
			"error", err)
		return err
	}
	defer file.Close()
	
	logLog().Info("Successfully rotated log files", 
		"main_log", logFilePath, 
		"backup_count", backupCount)
	
	return nil
}

// checkLogSize returns true if trimming is needed and the current file size
func checkLogSize(filePath string, maxSizeMB int) (bool, int64) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logLog().Error("Error checking log file size", "file", filePath, "error", err)
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

// Helper function to get basename
func basename(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}