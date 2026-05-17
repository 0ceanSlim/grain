package config

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
	"gopkg.in/fsnotify.v1"
)

// Self-write suppression. The admin write surface (NIP-86 methods
// like banpubkey, allowpubkey, etc.) mutates the same YAML / JSON
// files the watcher monitors. Without suppression every admin
// action would trigger a full server restart via the restart loop
// in server/startup.go — dropping every WebSocket connection
// including the one issuing the request.
//
// Pattern: right before saving a config file, the write helper
// calls SuppressWatcherFor(<path>) which records "ignore the next
// fs event for this path until <now + window>". The watcher
// consults the map and silently drops matched events. The window
// is generous enough to cover fsnotify's 1s debounce + the atomic
// tmp+rename + a little fs latency.
//
// A genuine *external* edit (operator hand-modifies whitelist.yml)
// still triggers a restart, as expected.
var (
	suppressMu sync.Mutex
	suppressed = make(map[string]time.Time) // canonical path → expiry
)

// suppressWindow gives the admin write enough headroom for the
// 1-second fsnotify debounce plus rename + flush. Two seconds is
// comfortable; longer windows make the watcher less responsive to
// real external edits when followed by admin writes, so we don't
// pad more than that.
const suppressWindow = 2 * time.Second

// SuppressWatcherFor blocks fsnotify-driven restarts for `path`
// for the next suppressWindow. Path is normalized with filepath.Clean
// so callers and the watcher always compare on the same key.
func SuppressWatcherFor(path string) {
	key := filepath.Clean(path)
	suppressMu.Lock()
	suppressed[key] = time.Now().Add(suppressWindow)
	suppressMu.Unlock()
}

// isSuppressed reports whether `path` is currently suppressed. It
// also opportunistically garbage-collects expired entries so the
// map can't grow without bound under sustained admin write load.
func isSuppressed(path string) bool {
	key := filepath.Clean(path)
	now := time.Now()
	suppressMu.Lock()
	defer suppressMu.Unlock()
	for k, exp := range suppressed {
		if !now.Before(exp) {
			delete(suppressed, k)
		}
	}
	exp, ok := suppressed[key]
	return ok && now.Before(exp)
}

func WatchConfigFile(filePath string, restartChan chan<- struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Config().Error("Error creating file watcher", "error", err)
		os.Exit(1) // Manually exit after logging the error
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		log.Config().Error("Failed to add file to watcher", "file", filePath, "error", err)
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
				// Skip self-writes by the admin API. The suppression
				// window covers fsnotify's debounce plus the atomic
				// rename used by the admin write helpers, so we
				// silently drop the event instead of firing a
				// restart.
				if isSuppressed(event.Name) {
					log.Config().Debug("Suppressed self-write event", "file", event.Name)
					continue
				}
				log.Config().Info("Config file modified", "file", filePath)
				if debounceTimer != nil {
					debounceTimer.Stop() // Cancel the previous timer
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					// One more check after the debounce in case an
					// admin write landed during the wait. Without
					// this, a sequence of (external write → admin
					// write within 1s) would still fire a restart
					// because the first event was queued before the
					// admin write registered suppression.
					if isSuppressed(filePath) {
						log.Config().Debug("Suppressed self-write event after debounce", "file", filePath)
						return
					}
					log.Config().Info("Config file change debounced, triggering restart", "file", filePath)
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
			log.Config().Error("Error watching file", "error", err)
		}
	}
}
