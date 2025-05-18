package utils

import (
	"log/slog"
	"runtime"

	configTypes "github.com/0ceanslim/grain/config/types"
)

// Constants based on your database analysis
const (
	// Conservative Note Size 
	BufferMessageSizeLimit = 128000 // 128 KiloBytes
)

var bufferlog = slog.Default().With("component", "buffer")

// CalculateOptimalBufferSize determines buffer size based on system resources
func CalculateOptimalBufferSize(cfg *configTypes.ServerConfig) int {
	// Get current memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Calculate total and available memory (in bytes)
	totalMemoryLimit := int64(cfg.ResourceLimits.MemoryMB) * 1024 * 1024
	currentMemoryUsage := int64(memStats.Sys)
	availableMemory := totalMemoryLimit - currentMemoryUsage
	
	// Calculate maximum connections based on config
	maxConnections := int64(cfg.Server.MaxConnections)
	
	// Reserve memory for other operations (75% of available)
	MemoryForBuffers := availableMemory * 25 / 100
	
	// Calculate per-connection memory budget
	memoryPerConnection := MemoryForBuffers / maxConnections
	
	// Calculate messages per connection buffer based on message size
	messagesPerBuffer := memoryPerConnection / BufferMessageSizeLimit
	
	// Apply reasonable bounds
	// Minimum: At least enough for a few messages
	minBufferSize := 5
	
	// Maximum: Cap at a reasonable number to prevent excessive memory use
	// With 128 KB messages, even 100 messages is ~12.8 MB per client
	maxBufferSize := 100
	
	// Apply bounds
	result := int(messagesPerBuffer)
	if result < minBufferSize {
		result = minBufferSize
		bufferlog.Warn("Buffer size increased to minimum", "size", minBufferSize)
	} else if result > maxBufferSize {
		result = maxBufferSize
		bufferlog.Debug("Buffer size capped at maximum", "size", maxBufferSize)
	}
	
	bufferlog.Info("Calculated buffer size", "messages", result, 
		"bytes_per_msg", BufferMessageSizeLimit, 
		"total_buffer_mb", float64(result)*float64(BufferMessageSizeLimit)/(1024*1024))
	
	return result
}