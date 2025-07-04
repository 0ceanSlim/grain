package config

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/0ceanslim/grain/server/utils/log"
)

var embeddedExamples embed.FS

// exampleFileMap maps target config filenames to their embedded example counterparts
var exampleFileMap = map[string]string{
	"config.yml":          "docs/examples/config.example.yml",
	"whitelist.yml":       "docs/examples/whitelist.example.yml",
	"blacklist.yml":       "docs/examples/blacklist.example.yml",
	"relay_metadata.json": "docs/examples/relay_metadata.example.json",
}

// SetEmbeddedExamples sets the embedded filesystem from main package
func SetEmbeddedExamples(fs embed.FS) {
	embeddedExamples = fs
}

// ensureConfigFile creates a config file from embedded example if it doesn't exist
func ensureConfigFile(targetPath string) error {
	log.Config().Debug("Checking if config file exists", "path", targetPath)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		examplePath, exists := exampleFileMap[filepath.Base(targetPath)]
		if !exists {
			log.Config().Error("No embedded example found for config file", "target", targetPath)
			return err
		}

		log.Config().Info("Config file does not exist, creating from embedded example",
			"target_path", targetPath,
			"example_path", examplePath)

		if err := extractEmbeddedFile(examplePath, targetPath); err != nil {
			log.Config().Error("Failed to extract embedded example file",
				"example_path", examplePath,
				"target_path", targetPath,
				"error", err)
			return err
		}

		log.Config().Info("Successfully created config file from embedded example", "path", targetPath)
	} else if err != nil {
		log.Config().Error("Error checking config file existence", "path", targetPath, "error", err)
		return err
	} else {
		log.Config().Debug("Config file already exists", "path", targetPath)
	}

	return nil
}

// extractEmbeddedFile reads an embedded file and writes it to the target path
func extractEmbeddedFile(embeddedPath, targetPath string) error {
	log.Config().Debug("Extracting embedded file", "source", embeddedPath, "destination", targetPath)

	data, err := embeddedExamples.ReadFile(embeddedPath)
	if err != nil {
		log.Config().Error("Failed to read embedded file", "source", embeddedPath, "error", err)
		return err
	}

	// Create target directory if it doesn't exist
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Config().Error("Failed to create target directory", "directory", targetDir, "error", err)
		return err
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		log.Config().Error("Failed to write config file", "destination", targetPath, "error", err)
		return err
	}

	log.Config().Info("Embedded file extracted successfully",
		"source", embeddedPath,
		"destination", targetPath,
		"bytes", len(data))

	return nil
}

// EnsureAllConfigFiles creates all default config files from embedded examples if they don't exist
func EnsureAllConfigFiles() error {
	log.Config().Debug("Ensuring all config files exist")

	for targetFile := range exampleFileMap {
		if err := ensureConfigFile(targetFile); err != nil {
			return err
		}
	}

	log.Config().Debug("All config files verified/created")
	return nil
}
