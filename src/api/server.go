package api

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dpanic/myjp/src/config"
	"github.com/dpanic/myjp/src/logger"

	"go.uber.org/zap"
)

// Server definition
type Server struct {
	config.Config
}

// New Server instance
func NewServer(config *config.Config) *Server {
	return &Server{
		*config,
	}
}

// Run starts listener and handles connections
func (server *Server) Run() {
	var (
		events = make(chan os.Signal, 1)
		done   = make(chan bool, 1)
	)
	signal.Notify(events,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	log := logger.Log.WithOptions(zap.Fields(
		zap.String("host", server.ListenHost),
		zap.Int("port", server.ListenPort),
	))
	log.Info("starting server")

	// starting server
	address := fmt.Sprintf("%s:%d", server.ListenHost, server.ListenPort)
	listener, err := net.Listen("tcp4", address)

	if err != nil {
		log.Error("error in listening to port",
			zap.Error(err),
		)
		return
	}
	defer listener.Close()

	// capture os signals
	go func() {
		for {
			s := <-events
			logger.Log.Warn(fmt.Sprintf("received %s", s.String()))

			switch s {
			case syscall.SIGHUP:
				// todo: you can implement configuration reload
				//

			case syscall.SIGINT:
				done <- true
				return
			case syscall.SIGTERM:
				done <- true
				return

			case syscall.SIGQUIT:
				done <- true
				return

			default:
				// noop
			}
		}
	}()

	// graceful shutdown
	go func() {
		log.Debug("waiting for shut down signal ^C")
		<-done

		log.Info("shut down signal received, closing connections...")
		listener.Close()
		os.Exit(0)
	}()

	// handling connections
	log.Info("started server")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Error("error in accepting connection",
				zap.Error(err),
			)
			continue
		}

		log.Debug("new client connection",
			zap.String("ip", conn.RemoteAddr().String()),
		)

		client := NewClient(&conn, server.RemoteHost, server.RemotePort)
		go client.handleRequest()
	}
}
