package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"runtime"
	"time"

	"github.com/dpanic/myjp/src/config"
	"github.com/dpanic/myjp/src/logger"
	"github.com/dpanic/myjp/src/pool"
	"github.com/dpanic/myjp/src/proxy"
	"go.uber.org/zap"
)

// Client ...
type Client struct {
	id            string
	born          time.Time
	conn          *net.Conn
	clientAddress string
	serverAddress string
	config.Config
	input    chan []byte
	output   chan []byte
	sections int
}

const (
	maxChanSize   = 256 << 10
	maxClientRead = 256 << 10
)

func NewClient(id string, conn *net.Conn, remoteHost string, remotePort int) (client *Client) {
	go pool.Instance.Connect(id, remoteHost, remotePort)

	clientAddress := (*conn).RemoteAddr().String()
	serverAddress := fmt.Sprintf("%s:%d", remoteHost, remotePort)

	client = &Client{
		id:            id,
		born:          time.Now(),
		input:         make(chan []byte, maxChanSize),
		output:        make(chan []byte, maxChanSize),
		clientAddress: clientAddress,
		serverAddress: serverAddress,
		conn:          conn,
	}

	client.Config = config.Config{
		RemoteHost: remoteHost,
		RemotePort: remotePort,
	}

	return
}

var (
	maxConnectionLifetime = 300 * time.Second
)

// handleRequest handles client request
func (client *Client) handleRequest() {
	log := logger.Log.WithOptions(zap.Fields(
		zap.String("clientAddress", client.clientAddress),
		zap.String("serverAddress", client.serverAddress),
	))

	var (
		reader           = bufio.NewReader(*client.conn)
		shouldWorkClient = make(chan bool, 128)
		shouldWorkProxy  = make(chan bool, 128)
		connection       *proxy.Connection
	)

	defer func() {
		runtime.GC()

		// close client connection
		(*client.conn).Close()

		// turn off proxy
		if connection != nil {
			for i := 0; i < connection.Sections; i++ {
				*connection.ShouldWork <- false
			}
		}

		// turn of client workers
		for i := 0; i < client.sections; i++ {
			shouldWorkClient <- false
		}

		log.Info("handle request is done",
			zap.Duration("duration", time.Since(client.born)),
		)
	}()

	// create context
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, maxConnectionLifetime)
	defer cancel()

	// client <-> proxy <-> server
	// create jump proxy
	go func() {
		defer func() {
			for i := 0; i < client.sections; i++ {
				shouldWorkClient <- false
			}
		}()

		var err error
		connection, err := proxy.New(client.id, client.RemoteHost, client.RemotePort)
		if err != nil {
			logger.Log.Error("error in creating remote connection",
				zap.Error(err))
			return
		}

		connection.Context = &ctx
		connection.Input = &client.input
		connection.Output = &client.output
		connection.ShouldWork = &shouldWorkProxy

		connection.SendWithContext()
	}()

	// client -> buffer
	// read from client
	go func() {
		client.sections++

		defer func() {
			log.Debug("shutting down client worker reader")
		}()

		for {
			line := make([]byte, maxClientRead)
			n, err := reader.Read(line)

			if err != nil && err != io.EOF {
				break
			}
			if err == io.EOF {
				break
			}

			line = line[:n]

			var (
				msg bytes.Buffer
			)

			select {
			case client.input <- line:
			case <-shouldWorkClient:
				return
			}

			if logger.Debug == "true" {
				msg.WriteString(fmt.Sprintf("Client Address: %q\n", client.clientAddress))
				msg.WriteString(fmt.Sprintf("Server Address: %q\n", client.serverAddress))
				msg.WriteString(fmt.Sprintf("Query: %q\n", string(line)))
				msg.WriteString("\n\n")
				logger.Enqueue(msg.String())
			}
		}
	}()

	// client <- buffer
	// send back to the client
	client.sections++

	for {
		select {
		case <-shouldWorkClient:
			log.Debug("shutting down client worker output")

			return

		case resp := <-client.output:
			var (
				msg bytes.Buffer
			)

			if logger.Debug == "true" {
				msg.WriteString(fmt.Sprintf("Client Address: %q\n", client.clientAddress))
				msg.WriteString(fmt.Sprintf("Server Address: %q\n", client.serverAddress))
				msg.WriteString(fmt.Sprintf("Response: %q\n", string(resp)))
				msg.WriteString("\n\n")
				logger.Enqueue(msg.String())
			}

			_, err := (*client.conn).Write(resp)
			if err != nil {
				log.Error("error in writing to the client",
					zap.Error(err),
				)
				return
			}

		case <-ctx.Done():
			log.Debug("context is done")

			return
		}
	}

	// client.conn.Write([]byte("Message received.\n"))
}
