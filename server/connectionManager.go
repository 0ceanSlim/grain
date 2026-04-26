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
	memoryThreshold:     1.5,             // DISABLED: isMemoryThresholdExceeded uses MemStats.Alloc/Sys, which is heap-utilization-after-GC and naturally sits at 80–95% for any healthy long-running Go program. With the prior 0.85 value the check fired ~10x/sec, evicting every newly-registered client (NOTICE "memory constraints") and making the relay appear unresponsive. Setting >1.0 disables eviction until the metric is replaced with real system/process memory pressure.
	estimatedMemPerConn: 2 * 1024 * 1024, // Start with 2MB estimate per connection
}

// RegisterConnection adds a connection to the manager. If the memory
// threshold is exceeded, the oldest connection is selected for eviction
// and closed AFTER releasing cm.mu — the close path re-enters this
// manager via RemoveConnection, which would self-deadlock the goroutine
// if we were still holding the lock here. That self-deadlock was the
// production WebSocket lockup symptom: the holding goroutine blocked
// forever on its own mutex, every subsequent new connection then blocked
// on cm.mu, and the relay went silent on WebSockets while HTTP kept
// serving (it never touches cm).
func (cm *ConnectionManager) RegisterConnection(client *Client) {
	cm.mu.Lock()

	cm.connections[client] = time.Now()

	var evict *Client
	if cm.isMemoryThresholdExceeded() {
		evict = cm.removeOldestLocked()
	}

	cm.mu.Unlock()

	if evict != nil {
		evictClient(evict)
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

// removeOldestLocked finds the oldest tracked connection, removes it
// from cm.connections, and returns it for eviction by the caller.
// MUST be called with cm.mu held. Returns nil if no connections exist.
//
// The split between removal-under-lock and the actual close (in
// evictClient, called WITHOUT the lock) is load-bearing: closing a
// client re-enters this manager via RemoveConnection, which would
// re-acquire cm.mu and deadlock the goroutine.
func (cm *ConnectionManager) removeOldestLocked() *Client {
	var oldestClient *Client
	var oldestTime time.Time

	for client, connTime := range cm.connections {
		if oldestClient == nil || connTime.Before(oldestTime) {
			oldestClient = client
			oldestTime = connTime
		}
	}

	if oldestClient == nil {
		return nil
	}

	log.RelayConnection().Info("Dropping oldest connection due to memory pressure",
		"client_id", oldestClient.id,
		"connected_since", oldestTime.Format(time.RFC3339),
		"age_seconds", time.Since(oldestTime).Seconds(),
		"total_connections", len(cm.connections))

	// Remove from tracking under the lock; the eviction path won't try
	// to remove again (and even if RemoveConnection is called, the
	// missing-key delete is a no-op).
	delete(cm.connections, oldestClient)

	return oldestClient
}

// evictClient performs the eviction of a client selected for memory-
// pressure dropping. MUST NOT be called while holding cm.mu — the
// close path re-enters this manager.
func evictClient(c *Client) {
	c.SendMessage([]interface{}{
		"NOTICE",
		"Disconnecting due to server memory constraints. Please reconnect.",
	})
	c.CloseClient()
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
