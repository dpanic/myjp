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

// Client represents a connected client with its connection state and communication channels.
// It manages the bidirectional communication between the client and the remote database server.
type Client struct {
	id            string      // Unique identifier for this client connection
	born          time.Time   // Timestamp when the client connection was established
	conn          *net.Conn   // Network connection to the client
	clientAddress string      // Remote address of the connected client
	serverAddress string      // Address of the remote database server
	config.Config             // Embedded configuration for remote host/port
	input         chan []byte // Channel for incoming data from client
	output        chan []byte // Channel for outgoing data to client
	sections      int         // Counter for active goroutine sections
}

const (
	// maxChanSize is the maximum buffer size for input/output channels.
	maxChanSize = 256 << 10
	// maxClientRead is the maximum bytes to read from client in one operation.
	maxClientRead = 256 << 10
)

// NewClient creates a new Client instance and initiates a connection to the remote host.
// It sets up input/output channels and configures the client with remote host information.
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
	// maxConnectionLifetime is the maximum duration a connection can remain active.
	maxConnectionLifetime = 300 * time.Second
)

// handleRequest manages the bidirectional data flow between the client and remote server.
// It creates a proxy connection, reads data from the client, and sends responses back.
// The function handles graceful shutdown and cleanup of all resources.
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
