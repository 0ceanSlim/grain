package config

import (
	"log"
	"runtime"
	"runtime/debug"
	"time"

	configTypes "github.com/0ceanslim/grain/config/types"
)

func SetResourceLimit(cfg *configTypes.ResourceLimits) {
	// Set CPU cores
	runtime.GOMAXPROCS(cfg.CPUCores)
	log.Printf("CPU cores set to %d\n", cfg.CPUCores)

	// Set maximum heap size
	if cfg.HeapSizeMB > 0 {
		heapSize := int64(uint64(cfg.HeapSizeMB) * 1024 * 1024)
		debug.SetMemoryLimit(heapSize)
		log.Printf("Heap size limited to %d MB\n", cfg.HeapSizeMB)
	}

	// Optional: Basic memory monitoring without goroutine management
	if cfg.MemoryMB > 0 {
		go monitorMemoryUsage(cfg.MemoryMB)
		log.Printf("Memory usage monitoring enabled at %d MB\n", cfg.MemoryMB)
	}
}

func monitorMemoryUsage(maxMemoryMB int) {
	for {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		usedMemoryMB := int(memStats.Alloc / 1024 / 1024)
		if usedMemoryMB > maxMemoryMB {
			log.Printf("Memory usage exceeded limit: %d MB used, limit is %d MB\n",
				usedMemoryMB, maxMemoryMB)
			debug.FreeOSMemory() // Attempt to free memory
		}

		time.Sleep(1 * time.Second)
	}
}
