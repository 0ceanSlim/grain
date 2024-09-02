package utils

import (
	"fmt"
	"log"
	"time"

	"gopkg.in/fsnotify.v1"
)

func WatchConfigFile(filePath string, restartChan chan<- struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Error creating file watcher:", err)
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		log.Fatal("Error adding file to watcher:", err)
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
				if debounceTimer != nil {
					debounceTimer.Stop() // Cancel the previous timer
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					fmt.Println("Config file modified, restarting server...")
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
			log.Println("Error watching file:", err)
		}
	}
}
