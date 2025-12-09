// Package pool provides a connection pool for managing TCP connections to remote database servers.
// It maintains pre-established connections that can be reused across multiple client requests.
package pool

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

// pool manages a collection of server connections indexed by a unique tag.
type pool struct {
	connections map[string]chan server // Map of connection channels keyed by id:host:port
}

var (
	// Instance is the global connection pool singleton.
	Instance = pool{
		connections: make(map[string]chan server, 0),
	}
)

// getTag generates a unique identifier string for a connection based on id, host, and port.
func (p *pool) getTag(id, host string, port int) (tag string) {
	tag = fmt.Sprintf("%s:%s:%d", id, host, port)
	return
}

// server represents a single pooled connection with its identifier and TCP connection.
type server struct {
	id   string       // Unique identifier for this connection
	conn *net.TCPConn // The underlying TCP connection
}

// Connect establishes a new connection to the remote host and adds it to the pool.
// If the connection fails, it logs the error and waits before returning.
func (p *pool) Connect(id, host string, port int) {
	tag := p.getTag(id, host, port)
	if _, ok := p.connections[tag]; !ok {
		p.connections[tag] = make(chan server, 10)
	}

	log := logger.Log.WithOptions(zap.Fields(
		zap.String("host", host),
		zap.Int("port", port),
	))

	conn, err := p.connectToRemoteHost(host, port)
	if err != nil {
		log.Error("error in connecting to remote host",
			zap.Error(err),
		)
		time.Sleep(2 * time.Second)
		return
	}

	log.Info("created new connection to remote host")
	p.connections[tag] <- server{
		id:   id,
		conn: conn,
	}
}

// connectToRemoteHost creates a new TCP connection to the specified host and port.
// It configures keep-alive settings on the connection.
// Returns the TCP connection and any error encountered.
func (p *pool) connectToRemoteHost(host string, port int) (rConn *net.TCPConn, err error) {
	address := fmt.Sprintf("%s:%d", host, port)
	rAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	rConn, err = net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		return nil, err
	}
	rConn.SetKeepAlive(true)
	rConn.SetKeepAlivePeriod(15 * time.Second)

	return
}

// Get retrieves a connection from the pool for the specified id, host, and port.
// It waits up to 5 seconds for an available connection before returning an error.
// Returns the TCP connection and any error encountered.
func (p *pool) Get(id, host string, port int) (rConn *net.TCPConn, err error) {
	tag := p.getTag(id, host, port)
	log := logger.Log.WithOptions(zap.Fields(
		zap.String("host", host),
		zap.Int("port", port),
	))

	if val, ok := p.connections[tag]; ok {
		select {

		case s := <-val:
			log.Info("got connection from pool",
				zap.String("id", s.id),
			)

			rConn = s.conn
			return

		case <-time.After(5 * time.Second):
			err = errors.New("no available connection for host:port")
			log.Error("error in getting host from pool",
				zap.Error(err),
			)

			return
		}
	}

	err = errors.New("host:port not found")
	log.Error("error in getting host from pool",
		zap.Error(err),
	)
	return

}
