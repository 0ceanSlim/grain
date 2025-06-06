package config

import (
	"runtime"
	"runtime/debug"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

func SetResourceLimit(cfg *cfgType.ResourceLimits) {
	// Set CPU cores
	runtime.GOMAXPROCS(cfg.CPUCores)
	log.Config().Info("CPU cores limit set", "cores", cfg.CPUCores)

	// Set maximum heap size
	if cfg.HeapSizeMB > 0 {
		heapSize := int64(uint64(cfg.HeapSizeMB) * 1024 * 1024)
		debug.SetMemoryLimit(heapSize)
		log.Config().Info("Heap size limit set", "size_mb", cfg.HeapSizeMB)
	}

	// Optional: Basic memory monitoring without goroutine management
	if cfg.MemoryMB > 0 {
		go monitorMemoryUsage(cfg.MemoryMB)
		log.Config().Info("Memory usage monitoring enabled", "limit_mb", cfg.MemoryMB)
	}
}

func monitorMemoryUsage(maxMemoryMB int) {
	for {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		usedMemoryMB := int(memStats.Alloc / 1024 / 1024)
		if usedMemoryMB > maxMemoryMB {
			log.Config().Warn("Memory usage exceeded limit", "used_mb", usedMemoryMB, "limit_mb", maxMemoryMB)
			debug.FreeOSMemory() // Attempt to free memory
		}

		time.Sleep(1 * time.Second)
	}
}
