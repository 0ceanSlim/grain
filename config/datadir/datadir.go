package datadir

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultDir returns the platform-appropriate default data directory for grain.
//   - Linux:   ~/.grain/
//   - macOS:   ~/Library/Application Support/grain/
//   - Windows: %APPDATA%\grain\
func DefaultDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return ".grain"
		}
		return filepath.Join(home, "Library", "Application Support", "grain")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "grain")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return ".grain"
		}
		return filepath.Join(home, "AppData", "Roaming", "grain")
	default: // linux and other unix
		home, err := os.UserHomeDir()
		if err != nil {
			return ".grain"
		}
		return filepath.Join(home, ".grain")
	}
}

// Resolve determines the data directory using this precedence:
//  1. flagValue (--data-dir CLI flag) if non-empty
//  2. GRAIN_DATA_DIR environment variable if set
//  3. Platform default via DefaultDir()
func Resolve(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envDir := os.Getenv("GRAIN_DATA_DIR"); envDir != "" {
		return envDir
	}
	return DefaultDir()
}

// EnsureExists creates the data directory and standard subdirectories if they don't exist.
func EnsureExists(dir string) error {
	subdirs := []string{
		"",     // the data dir itself
		"data", // nostrdb database files
	}
	for _, sub := range subdirs {
		path := filepath.Join(dir, sub)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}
