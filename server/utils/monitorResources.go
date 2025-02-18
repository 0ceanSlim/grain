package utils

import (
	"runtime"
)

// GetCurrentMemoryUsageMB returns the current total memory usage in megabytes
func GetCurrentMemoryUsageMB() float64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Convert bytes to megabytes
	return float64(memStats.Sys) / 1024 / 1024
}

// GetCurrentHeapUsageMB returns the current heap memory usage in megabytes
func GetCurrentHeapUsageMB() float64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// HeapAlloc represents bytes of allocated heap objects
	return float64(memStats.HeapAlloc) / 1024 / 1024
}
