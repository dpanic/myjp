// Package main is the entry point for myjp, a MySQL/MariaDB JSON proxy server.
// It creates necessary directories and starts the API server to handle database connections.
package main

import (
	"os"

	"github.com/dpanic/myjp/src/api"
)

// main creates the logs directory if it doesn't exist and starts the API server.
func main() {
	os.MkdirAll("./logs", 0755)

	api.Run()
}
