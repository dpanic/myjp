// Package config handles loading and parsing of myjp configuration files.
// It reads proxy mappings from /etc/myjp.conf that define listen and remote host/port pairs.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/dpanic/myjp/src/logger"
)

const (
	// configFileLoc is the default path to the myjp configuration file.
	configFileLoc = "/etc/myjp.conf"
)

// Config represents a single proxy configuration with listen and remote host/port pairs.
type Config struct {
	ListenHost string // Host address to listen on for incoming connections
	ListenPort int    // Port to listen on for incoming connections
	RemoteHost string // Remote database server host to proxy to
	RemotePort int    // Remote database server port to proxy to
}

// LoadAll reads and parses all proxy configurations from the config file.
// Each line in the config file should be in format: listen_host:listen_port remote_host:remote_port
// Returns a slice of Config pointers and any error encountered.
func LoadAll() (configs []*Config, err error) {
	file, err := os.Open(configFileLoc)
	if err != nil {
		return
	}
	defer file.Close()

	var (
		reader = bufio.NewReader(file)
		unique = make(map[string]*Config, 0)
	)

	for {
		raw, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			break
		}
		if err == io.EOF {
			break
		}

		line := string(raw)
		line = strings.Trim(line, "\r")
		line = strings.Trim(line, "\n")
		line = strings.Trim(line, "\t")
		line = strings.ReplaceAll(line, ":", " ")

		parts := strings.Split(line, " ")
		if len(parts) < 4 {
			logger.Log.Error("wrong configuration")
			continue
		}

		listenHost := parts[0]
		listenPort, _ := strconv.Atoi(parts[1])
		remoteHost := parts[2]
		remotePort, _ := strconv.Atoi(parts[3])
		tag := fmt.Sprintf("%s:%d", listenHost, listenPort)

		unique[tag] = &Config{
			ListenHost: listenHost,
			ListenPort: listenPort,
			RemoteHost: remoteHost,
			RemotePort: remotePort,
		}
	}

	configs = make([]*Config, 0)
	for _, config := range unique {
		configs = append(configs, config)
		// fmt.Println(config)
	}

	return
}
