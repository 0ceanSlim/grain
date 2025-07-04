package server

import (
	"runtime"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// ConnectionManager tracks connections and memory usage
type ConnectionManager struct {
	connections         map[*Client]time.Time
	memoryThreshold     float64 // percentage (0.0-1.0)
	estimatedMemPerConn int64   // in bytes
	mu                  sync.Mutex
}

// Global connection manager instance
var connManager = &ConnectionManager{
	connections:         make(map[*Client]time.Time),
	memoryThreshold:     0.85,            // 85% memory threshold
	estimatedMemPerConn: 2 * 1024 * 1024, // Start with 2MB estimate per connection
}

// RegisterConnection adds a connection to the manager
func (cm *ConnectionManager) RegisterConnection(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.connections[client] = time.Now()

	// Check memory usage after adding
	if cm.isMemoryThresholdExceeded() {
		cm.dropOldestConnection()
	}
}

// RemoveConnection removes a connection from tracking
func (cm *ConnectionManager) RemoveConnection(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.connections, client)
}

// isMemoryThresholdExceeded checks if memory usage exceeds threshold
func (cm *ConnectionManager) isMemoryThresholdExceeded() bool {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get percentage of memory used
	memoryUsed := float64(memStats.Alloc) / float64(memStats.Sys)

	if memoryUsed > cm.memoryThreshold {
		log.RelayConnection().Warn("Memory threshold exceeded",
			"memory_used_pct", memoryUsed*100,
			"threshold_pct", cm.memoryThreshold*100,
			"connections", len(cm.connections))
		return true
	}

	return false
}

// dropOldestConnection drops the oldest connection
func (cm *ConnectionManager) dropOldestConnection() {
	var oldestClient *Client
	var oldestTime time.Time

	// Find the oldest connection
	for client, connTime := range cm.connections {
		if oldestClient == nil || connTime.Before(oldestTime) {
			oldestClient = client
			oldestTime = connTime
		}
	}

	if oldestClient != nil {
		log.RelayConnection().Info("Dropping oldest connection due to memory pressure",
			"client_id", oldestClient.id,
			"connected_since", oldestTime.Format(time.RFC3339),
			"age_seconds", time.Since(oldestTime).Seconds(),
			"total_connections", len(cm.connections))

		// Send notice to client before disconnecting
		oldestClient.SendMessage([]interface{}{
			"NOTICE",
			"Disconnecting due to server memory constraints. Please reconnect.",
		})

		// Remove from tracking first to prevent recursion
		delete(cm.connections, oldestClient)

		// Close the client connection
		oldestClient.CloseClient()
	}
}

// GetConnectionCount returns the current number of connections
func (cm *ConnectionManager) GetConnectionCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.connections)
}

// GetMemoryStats returns memory statistics for monitoring
func (cm *ConnectionManager) GetMemoryStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	cm.mu.Lock()
	connCount := len(cm.connections)
	cm.mu.Unlock()

	return map[string]interface{}{
		"memory_used_bytes":         memStats.Alloc,
		"memory_total_bytes":        memStats.Sys,
		"memory_used_percent":       float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		"connections":               connCount,
		"estimated_mem_per_conn_mb": float64(cm.estimatedMemPerConn) / (1024 * 1024),
	}
}
