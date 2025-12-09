// Package proxy provides functionality for proxying data between clients and remote database servers.
// It handles bidirectional communication, connection timeouts, and graceful shutdown.
package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dpanic/myjp/src/logger"
	"github.com/dpanic/myjp/src/pool"
	"github.com/dpanic/myjp/src/stats"
	"go.uber.org/zap"
)

var (
	// mutex protects concurrent access to shared proxy resources.
	mutex = &sync.Mutex{}
)

// Connection represents a proxy connection to a remote database server.
// It manages the bidirectional data flow and tracks connection state.
type Connection struct {
	rConn        *net.TCPConn      // TCP connection to the remote server
	id           string            // Unique identifier for this connection
	host         string            // Remote server host
	port         int               // Remote server port
	Sections     int               // Counter for active goroutine sections
	mutex        sync.Mutex        // Mutex for protecting lastActivity
	lastActivity time.Time         // Timestamp of last activity on this connection
	Context      *context.Context  // Context for cancellation
	ShouldWork   *chan bool        // Channel for signaling shutdown
	Input        *chan []byte      // Channel for incoming data from client
	Output       *chan []byte      // Channel for outgoing data to client
}

// New creates a new proxy Connection to the specified host and port.
// It retrieves a connection from the pool and updates connection statistics.
// Returns the Connection and any error encountered.
func New(id, host string, port int) (connection *Connection, err error) {
	stats.Instance.IncActiveConnections()
	stats.Instance.IncConnections()
	stats.Instance.AddConnectionID(id)

	rConn, err := pool.Instance.Get(id, host, port)
	if err != nil {
		logger.Log.Error("error in creating new connection",
			zap.String("host", host),
			zap.Int("port", port),
			zap.Error(err),
		)
		return
	}

	connection = &Connection{
		id:           id,
		host:         host,
		port:         port,
		rConn:        rConn,
		lastActivity: time.Now(),
	}

	return
}

const (
	// maxServerRead is the maximum bytes to read from server in one operation.
	maxServerRead = 256 << 10
	// maxIdleTime is the maximum time a connection can be idle before timeout.
	maxIdleTime = 60 * time.Second
)

// SendWithContext manages the bidirectional data flow between client and remote server.
// It spawns goroutines to handle sending data to the server and receiving responses.
// The function monitors for idle timeout and handles graceful shutdown.
// Returns any error encountered during communication.
func (connection *Connection) SendWithContext() (err error) {
	log := logger.Log.WithOptions(zap.Fields(
		zap.String("id", connection.id),
		zap.String("host", connection.host),
		zap.Int("port", connection.port),
	))

	defer func() {
		log.Debug("shutting down proxy")

		(*connection.Context).Done()

		// turn off server connection
		(*connection.rConn).Close()

		for i := 0; i < connection.Sections; i++ {
			*connection.ShouldWork <- false
		}

		stats.Instance.DecActiveConnections()
		stats.Instance.DelConnectionID(connection.id)
	}()

	// client -> proxy -> server
	go func() {
		connection.Sections++

		defer func() {
			log.Debug("shutting down proxy worker client read")
		}()

		for {
			var data []byte
			select {
			case data = <-*connection.Input:
			case <-*connection.ShouldWork:
				return
			}

			connection.mutex.Lock()
			connection.lastActivity = time.Now()
			connection.mutex.Unlock()

			_, err := connection.rConn.Write(data)

			if err != nil {
				log.Error("error in sending data")
				return
			}

			// log.Debug("sent data to remote server",
			// 	zap.Int("bytes", total),
			// )
		}
	}()

	// server -> proxy -> client
	// read from server and send back to client
	go func() {
		defer func() {
			log.Debug("shutting down proxy worker server read")
		}()

		connection.Sections++

		var (
			reader = bufio.NewReader(connection.rConn)
		)

		for {
			line := make([]byte, maxServerRead)
			total, err := reader.Read(line)

			connection.mutex.Lock()
			connection.lastActivity = time.Now()
			connection.mutex.Unlock()

			if err != nil && err != io.EOF {
				return
			}

			if err == io.EOF {
				return
			}

			line = line[:total]
			*connection.Output <- line

			// log.Debug("received data from remote server",
			// 	zap.Int("bytes", total),
			// )
		}
	}()

	isTimeouted := false
	connection.Sections++
	for {
		select {
		case <-*connection.ShouldWork:
			return

		case <-time.After(time.Second):
			connection.mutex.Lock()
			if time.Since(connection.lastActivity) > maxIdleTime {
				isTimeouted = true
			}
			connection.mutex.Unlock()

			if isTimeouted {
				log.Warn("connection to remote server timeouted")
				err = errors.New("connection to remote server timeouted")
				return
			}
		}
	}
}
