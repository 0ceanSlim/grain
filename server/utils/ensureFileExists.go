package utils

import (
	"os"

	"github.com/0ceanslim/grain/server/utils/log"
)

// EnsureFileExists creates a file from embedded content if it doesn't exist
func EnsureFileExists(filePath string, content []byte) {
	log.Util().Debug("Checking if file exists", "path", filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Util().Info("File does not exist, creating from embedded content",
			"target_path", filePath)

		err = writeEmbeddedFile(filePath, content)
		if err != nil {
			log.Util().Error("Failed to write embedded file",
				"target_path", filePath,
				"error", err)
			panic(err)
		}

		log.Util().Info("Successfully created file from embedded content",
			"path", filePath)
	} else if err != nil {
		log.Util().Error("Error checking file existence",
			"path", filePath,
			"error", err)
	} else {
		log.Util().Debug("File already exists", "path", filePath)
	}
}

// writeEmbeddedFile writes embedded content to a file
func writeEmbeddedFile(filePath string, content []byte) error {
	log.Util().Debug("Writing embedded content to file", "path", filePath, "size_bytes", len(content))

	file, err := os.Create(filePath)
	if err != nil {
		log.Util().Error("Failed to create file", "path", filePath, "error", err)
		return err
	}
	defer file.Close()

	bytesWritten, err := file.Write(content)
	if err != nil {
		log.Util().Error("Failed to write content to file", "path", filePath, "error", err)
		return err
	}

	log.Util().Info("Embedded file written successfully", "path", filePath, "bytes", bytesWritten)
	return nil
}
