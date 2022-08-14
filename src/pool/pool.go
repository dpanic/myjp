package pool

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

type pool struct {
	connections map[string]chan server
}

var (
	Instance = pool{
		connections: make(map[string]chan server, 0),
	}
)

func (p *pool) getTag(id, host string, port int) (tag string) {
	tag = fmt.Sprintf("%s:%s:%d", id, host, port)
	return
}

type server struct {
	id   string
	conn *net.TCPConn
}

// Connect adds connections to the pool
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

// connectToRemoteHost creates new connection to the pool
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

// Get host:port *net.TCPConn from connection pool
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
