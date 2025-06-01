package utils

import (
	"os"

	"github.com/0ceanslim/grain/server/utils/log"
)

func EnsureFileExists(filePath, examplePath string) {
	log.Util().Debug("Checking if file exists", "path", filePath)
	
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Util().Info("File does not exist, creating from example", 
			"target_path", filePath, 
			"example_path", examplePath)
		
		err = copyFile(examplePath, filePath)
		if err != nil {
			log.Util().Error("Failed to copy example file", 
				"example_path", examplePath, 
				"target_path", filePath, 
				"error", err)
			// Instead of log.Fatalf which exits the program immediately,
			// we'll log the error and then panic to allow for proper cleanup
			panic(err)
		}
		
		log.Util().Info("Successfully created file from example", 
			"path", filePath)
	} else if err != nil {
		// Handle other potential errors from os.Stat
		log.Util().Error("Error checking file existence", 
			"path", filePath, 
			"error", err)
	} else {
		log.Util().Debug("File already exists", "path", filePath)
	}
}