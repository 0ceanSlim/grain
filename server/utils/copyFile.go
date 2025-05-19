package utils

import (
	"io"
	"os"
	"path/filepath"
)

func copyFile(src, dst string) error {
	utilLog().Debug("Copying file", "source", src, "destination", dst)
	
	sourceFile, err := os.Open(src)
	if err != nil {
		utilLog().Error("Failed to open source file", "source", src, "error", err)
		return err
	}
	defer sourceFile.Close()

	// Create the directory for the destination file if it doesn't exist
	destDir := filepath.Dir(dst)
	err = os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		utilLog().Error("Failed to create destination directory", "directory", destDir, "error", err)
		return err
	}

	destinationFile, err := os.Create(dst)
	if err != nil {
		utilLog().Error("Failed to create destination file", "destination", dst, "error", err)
		return err
	}
	defer destinationFile.Close()

	bytesWritten, err := io.Copy(destinationFile, sourceFile)
	if err != nil {
		utilLog().Error("Failed to copy file contents", "source", src, "destination", dst, "error", err)
		return err
	}
	
	utilLog().Info("File copied successfully", "source", src, "destination", dst, "bytes", bytesWritten)
	return nil
}