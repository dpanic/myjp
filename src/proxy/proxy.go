package proxy

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dpanic/myjp/src/logger"
	"github.com/dpanic/myjp/src/stats"
	"go.uber.org/zap"
)

var (
	mutex = &sync.Mutex{}
)

type Connection struct {
	rConn *net.TCPConn
	id    string
	host  string
	port  int

	Context *context.Context
	Done    *chan bool
	Input   *chan []byte
	Output  *chan []byte
}

// genID generates random ID
func genID() string {
	raw := fmt.Sprintf("%v", time.Now().UnixNano())

	h := sha256.New()
	h.Write([]byte(raw))
	bs := string(h.Sum(nil))

	return fmt.Sprintf("%x", bs[0:7])
}

// creates new connection
func New(host string, port int) (connection *Connection, err error) {
	id := genID()

	stats.Instance.IncActiveConnections()
	stats.Instance.IncConnections()
	stats.Instance.AddConnectionID(id)

	rConn, err := connectToRemoteHost(host, port)
	if err != nil {
		logger.Log.Error("error in creating new connection",
			zap.String("host", host),
			zap.Int("port", port),
			zap.Error(err),
		)
		return
	}

	connection = &Connection{
		id:    id,
		host:  host,
		port:  port,
		rConn: rConn,
	}

	return
}

// connectToRemoteHost creates new connection to the pool
func connectToRemoteHost(host string, port int) (rConn *net.TCPConn, err error) {
	address := fmt.Sprintf("%s:%d", host, port)
	rAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	rConn, err = net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		return nil, err
	}

	return
}

const (
	maxServerRead = 256 * 1024
)

// SendWithContext data to the server
func (connection *Connection) SendWithContext() (err error) {
	defer func() {
		(*connection.Context).Done()

		stats.Instance.DecActiveConnections()
		stats.Instance.DelConnectionID(connection.id)
	}()

	log := logger.Log.WithOptions(zap.Fields(
		zap.String("id", connection.id),
		zap.String("host", connection.host),
		zap.Int("port", connection.port),
	))

	// send client data to remote server
	go func() {
		for {
			data := <-*connection.Input

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

	// read from server and send back to client
	go func() {
		var (
			reader = bufio.NewReader(connection.rConn)
		)

		for {
			line := make([]byte, maxServerRead)
			total, err := reader.Read(line)

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

	<-*connection.Done
	return
}
