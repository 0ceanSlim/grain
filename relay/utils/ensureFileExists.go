package utils

import (
	"log"
	"os"
)

func EnsureFileExists(filePath, examplePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		err = copyFile(examplePath, filePath)
		if err != nil {
			log.Fatalf("Failed to copy %s to %s: %v", examplePath, filePath, err)
		}
	}
}