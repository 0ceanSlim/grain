package config

import (
	"os"
	"time"

	"gopkg.in/fsnotify.v1"
)

func WatchConfigFile(filePath string, restartChan chan<- struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		configLog().Error("Error creating file watcher", "error", err)
		os.Exit(1) // Manually exit after logging the error
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		configLog().Error("Failed to add file to watcher", "file", filePath, "error", err)
		os.Exit(1) // Manually exit after logging the error
	}

	var debounceTimer *time.Timer
	debounceDuration := 1 * time.Second // Adjust this duration as needed

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				configLog().Info("Config file modified", "file", filePath)
				if debounceTimer != nil {
					debounceTimer.Stop() // Cancel the previous timer
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					configLog().Info("Config file change debounced, triggering restart", "file", filePath)
					select {
					case restartChan <- struct{}{}:
					default:
						// Skip sending restart signal if there's already one in the channel
					}
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			configLog().Error("Error watching file", "error", err)
		}
	}
}
