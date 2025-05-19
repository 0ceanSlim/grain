package utils

import (
	"os"
)

func EnsureFileExists(filePath, examplePath string) {
	utilLog().Debug("Checking if file exists", "path", filePath)
	
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utilLog().Info("File does not exist, creating from example", 
			"target_path", filePath, 
			"example_path", examplePath)
		
		err = copyFile(examplePath, filePath)
		if err != nil {
			utilLog().Error("Failed to copy example file", 
				"example_path", examplePath, 
				"target_path", filePath, 
				"error", err)
			// Instead of log.Fatalf which exits the program immediately,
			// we'll log the error and then panic to allow for proper cleanup
			panic(err)
		}
		
		utilLog().Info("Successfully created file from example", 
			"path", filePath)
	} else if err != nil {
		// Handle other potential errors from os.Stat
		utilLog().Error("Error checking file existence", 
			"path", filePath, 
			"error", err)
	} else {
		utilLog().Debug("File already exists", "path", filePath)
	}
}