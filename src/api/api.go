// Package api provides the HTTP API server and client handling for the MySQL/MariaDB proxy.
// It manages server lifecycle, client connections, and proxying requests to remote database servers.
package api

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/dpanic/myjp/src/config"
	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

// Run loads all configurations and starts a server instance for each configuration.
// It blocks indefinitely after starting all servers.
func Run() {
	configs, err := config.LoadAll()

	if err != nil {
		logger.Log.Panic("error in reading configuration",
			zap.Error(err),
		)
	}

	for _, config := range configs {
		server := NewServer(config)
		go server.Run()
	}

	select {}
}

// genID generates a unique identifier based on the current nanosecond timestamp.
// It returns a truncated SHA-256 hash as a hexadecimal string.
func genID() string {
	raw := fmt.Sprintf("%v", time.Now().UnixNano())

	h := sha256.New()
	h.Write([]byte(raw))
	bs := string(h.Sum(nil))

	return fmt.Sprintf("%x", bs[0:7])
}
