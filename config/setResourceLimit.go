package config

import (
	"fmt"
	//"log"
	"runtime"
	"runtime/debug"
	"time"

	configTypes "github.com/0ceanslim/grain/config/types"
)

func SetResourceLimit(cfg *configTypes.ResourceLimits) {
	// Set CPU cores
	runtime.GOMAXPROCS(cfg.CPUCores)
	log.Info(fmt.Sprintf("CPU cores set to %d", cfg.CPUCores))

	// Set maximum heap size
	if cfg.HeapSizeMB > 0 {
		heapSize := int64(uint64(cfg.HeapSizeMB) * 1024 * 1024)
		debug.SetMemoryLimit(heapSize)
		log.Info(fmt.Sprintf("Heap size limited to %d MB", cfg.HeapSizeMB))
	}

	// Optional: Basic memory monitoring without goroutine management
	if cfg.MemoryMB > 0 {
		go monitorMemoryUsage(cfg.MemoryMB)
		log.Info(fmt.Sprintf("Memory usage monitoring enabled at %d MB", cfg.MemoryMB))
	}
}

func monitorMemoryUsage(maxMemoryMB int) {
	for {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		usedMemoryMB := int(memStats.Alloc / 1024 / 1024)
		if usedMemoryMB > maxMemoryMB {
			log.Warn(fmt.Sprintf("Memory usage exceeded: %d MB used, limit is %d MB", usedMemoryMB, maxMemoryMB))
			debug.FreeOSMemory() // Attempt to free memory
		}

		time.Sleep(1 * time.Second)
	}
}
