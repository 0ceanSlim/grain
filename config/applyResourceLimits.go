package config

import (
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	configTypes "grain/config/types"
)

var (
	maxGoroutinesChan   chan struct{}
	wg                  sync.WaitGroup
	goroutineQueue      []func()
	goroutineQueueMutex sync.Mutex
)

func ApplyResourceLimits(cfg *configTypes.ResourceLimits) {
	// Set CPU cores
	runtime.GOMAXPROCS(cfg.CPUCores)

	// Set maximum heap size
	if cfg.HeapSizeMB > 0 {
		heapSize := int64(uint64(cfg.HeapSizeMB) * 1024 * 1024)
		debug.SetMemoryLimit(heapSize)
		log.Printf("Heap size limited to %d MB\n", cfg.HeapSizeMB)
	}

	// Start monitoring memory usage
	if cfg.MemoryMB > 0 {
		go monitorMemoryUsage(cfg.MemoryMB)
		log.Printf("Max memory usage limited to %d MB\n", cfg.MemoryMB)
	}

	// Set maximum number of Go routines
	if cfg.MaxGoroutines > 0 {
		maxGoroutinesChan = make(chan struct{}, cfg.MaxGoroutines)
		log.Printf("Max goroutines limited to %d\n", cfg.MaxGoroutines)
	}
}

// LimitedGoRoutine starts a goroutine with limit enforcement
func LimitedGoRoutine(f func()) {
	// By default, all routines are considered critical
	goroutineQueueMutex.Lock()
	goroutineQueue = append(goroutineQueue, f)
	goroutineQueueMutex.Unlock()
	attemptToStartGoroutine()
}

func attemptToStartGoroutine() {
	goroutineQueueMutex.Lock()
	defer goroutineQueueMutex.Unlock()

	if len(goroutineQueue) > 0 {
		select {
		case maxGoroutinesChan <- struct{}{}:
			wg.Add(1)
			go func(f func()) {
				defer func() {
					wg.Done()
					<-maxGoroutinesChan
					attemptToStartGoroutine()
				}()
				f()
			}(goroutineQueue[0])

			// Remove the started goroutine from the queue
			goroutineQueue = goroutineQueue[1:]

		default:
			// If the channel is full, consider dropping the oldest non-critical goroutine
			dropOldestNonCriticalGoroutine()
		}
	}
}

func dropOldestNonCriticalGoroutine() {
	goroutineQueueMutex.Lock()
	defer goroutineQueueMutex.Unlock()

	if len(goroutineQueue) > 0 {
		log.Println("Dropping the oldest non-critical goroutine to free resources.")
		goroutineQueue = goroutineQueue[1:]
		attemptToStartGoroutine()
	}
}

func WaitForGoroutines() {
	wg.Wait()
}

func monitorMemoryUsage(maxMemoryMB int) {
	for {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		usedMemoryMB := int(memStats.Alloc / 1024 / 1024)
		if usedMemoryMB > maxMemoryMB {
			log.Printf("Memory usage exceeded limit: %d MB used, limit is %d MB\n", usedMemoryMB, maxMemoryMB)
			debug.FreeOSMemory() // Attempt to free memory

			// If memory usage is still high, attempt to drop non-critical goroutines
			if usedMemoryMB > maxMemoryMB {
				dropOldestNonCriticalGoroutine()
			}
		}

		time.Sleep(1 * time.Second)
	}
}
