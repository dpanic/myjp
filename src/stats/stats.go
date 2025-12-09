// Package stats provides connection statistics tracking for the proxy server.
// It maintains counters for active connections, total connections, and request counts.
package stats

import (
	"sync"
	"time"

	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

// Stats holds the connection statistics for the proxy server.
type Stats struct {
	ActiveConnections int64    // Number of currently active connections
	Connections       int64    // Total number of connections established
	Requests          int64    // Total number of requests processed
	ConnectionIDS     []string // List of active connection identifiers
}

var (
	// Instance is the global stats singleton.
	Instance *Stats
	// mutex protects concurrent access to stats.
	mutex = &sync.Mutex{}
)

func init() {
	Instance = &Stats{
		ConnectionIDS: make([]string, 0),
	}

	go func() {
		for {
			mutex.Lock()

			logger.Log.Debug("global stats",
				zap.Int64("connections", Instance.Connections),
				zap.Strings("connectionIDS", Instance.ConnectionIDS),
				zap.Int64("activeConnections", Instance.ActiveConnections),
				zap.Int64("requests", Instance.Requests),
			)
			mutex.Unlock()

			time.Sleep(60 * time.Second)
		}
	}()
}

// IncActiveConnections increments the active connections counter by one.
func (stats *Stats) IncActiveConnections() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.ActiveConnections += 1
}

// DecActiveConnections decrements the active connections counter by one.
func (stats *Stats) DecActiveConnections() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.ActiveConnections -= 1
}

// IncRequests increments the requests counter by one.
func (stats *Stats) IncRequests() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.Requests += 1
}

// IncConnections increments the total connections counter by one.
func (stats *Stats) IncConnections() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.Connections += 1
}

// AddConnectionID adds a connection identifier to the list of active connections.
func (stats *Stats) AddConnectionID(id string) {
	mutex.Lock()
	defer mutex.Unlock()

	stats.ConnectionIDS = append(stats.ConnectionIDS, id)
}

// DelConnectionID removes a connection identifier from the list of active connections.
// It swaps the element to remove with the last element and truncates the slice.
func (stats *Stats) DelConnectionID(id string) {
	mutex.Lock()
	defer mutex.Unlock()

	idx := -1
	for i := 0; i < len(stats.ConnectionIDS); i++ {
		if stats.ConnectionIDS[i] == id {
			idx = i
			break
		}
	}
	stats.ConnectionIDS[idx] = stats.ConnectionIDS[len(stats.ConnectionIDS)-1]
	stats.ConnectionIDS = stats.ConnectionIDS[:len(stats.ConnectionIDS)-1]
}
