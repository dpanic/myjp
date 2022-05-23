package pool

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dpanic/myjp/src/config"
	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

type pool struct {
	servers map[string]chan *net.TCPConn
}

var (
	Instance = pool{
		servers: make(map[string]chan *net.TCPConn, 0),
	}
)

func (p *pool) getTag(host string, port int) (tag string) {
	tag = fmt.Sprintf("%s:%d", host, port)
	return
}

// add server config to the pool
func (p *pool) Add(config *config.Config) {
	tag := p.getTag(config.RemoteHost, config.RemotePort)
	Instance.servers[tag] = make(chan *net.TCPConn, 2)

	go p.Connect(config.RemoteHost, config.RemotePort)
}

// Connect adds connections to the pool
func (p *pool) Connect(host string, port int) {
	log := logger.Log.WithOptions(zap.Fields(
		zap.String("host", host),
		zap.Int("port", port),
	))
	tag := p.getTag(host, port)

	for {
		conn, err := p.connectToRemoteHost(host, port)
		if err != nil {
			log.Error("error in connecting to remote host",
				zap.Error(err),
			)
			time.Sleep(2 * time.Second)
			continue
		}
		log.Info("created new connection to remote host")
		p.servers[tag] <- conn
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
func (p *pool) Get(host string, port int) (rConn *net.TCPConn, err error) {
	tag := p.getTag(host, port)
	log := logger.Log.WithOptions(zap.Fields(
		zap.String("host", host),
		zap.Int("port", port),
	))

	if val, ok := p.servers[tag]; ok {
		select {
		case rConn = <-val:
			log.Info("got connection from pool")
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
