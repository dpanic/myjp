package stats

import (
	"sync"
	"time"

	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

type Stats struct {
	ActiveConnections int64
	Connections       int64
	Requests          int64
	ConnectionIDS     []string
}

var (
	Instance *Stats
	mutex    = &sync.Mutex{}
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

func (stats *Stats) IncActiveConnections() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.ActiveConnections += 1
}

func (stats *Stats) DecActiveConnections() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.ActiveConnections -= 1
}

func (stats *Stats) IncRequests() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.Requests += 1
}

func (stats *Stats) IncConnections() {
	mutex.Lock()
	defer mutex.Unlock()

	stats.Connections += 1
}

func (stats *Stats) AddConnectionID(id string) {
	mutex.Lock()
	defer mutex.Unlock()

	stats.ConnectionIDS = append(stats.ConnectionIDS, id)
}

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
