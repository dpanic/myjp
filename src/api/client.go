package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/dpanic/myjp/src/config"
	"github.com/dpanic/myjp/src/logger"
	"github.com/dpanic/myjp/src/proxy"
	"go.uber.org/zap"
)

// Client ...
type Client struct {
	conn          *net.Conn
	clientAddress string
	serverAddress string
	config.Config
	input  chan []byte
	output chan []byte
}

const (
	maxChanSize   = 256 * 1024
	maxClientRead = 256 * 1024
)

func NewClient(conn *net.Conn, remoteHost string, remotePort int) (client *Client) {
	clientAddress := (*conn).RemoteAddr().String()
	serverAddress := fmt.Sprintf("%s:%d", remoteHost, remotePort)

	client = &Client{
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
	maxConnectionLifetime = 120 * time.Second
)

// handleRequest handles client request
func (client *Client) handleRequest() {
	defer func() {
		(*client.conn).Close()
	}()

	var (
		reader = bufio.NewReader(*client.conn)
		done   = make(chan bool)
	)

	// create context
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, maxConnectionLifetime)
	defer cancel()

	// create jump proxy
	go func() {
		connection, err := proxy.New(client.RemoteHost, client.RemotePort)
		if err != nil {
			logger.Log.Error("error in creating remote connection",
				zap.Error(err))
			return
		}

		connection.Context = &ctx
		connection.Input = &client.input
		connection.Output = &client.output
		connection.Done = &done

		err = connection.SendWithContext()
		if err != nil {
			logger.Log.Error("error in sending data to remote proxy",
				zap.Error(err),
			)
		}
	}()

	log := logger.Log.WithOptions(zap.Fields(
		zap.String("listenHost", client.ListenHost),
		zap.Int("listenPort", client.ListenPort),
		zap.String("RemoteHost", client.RemoteHost),
		zap.Int("remotePort", client.RemotePort),
	))

	// read from client
	go func() {
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

			client.input <- line

			if logger.Debug == "true" {
				msg.WriteString(fmt.Sprintf("Client Address: %q\n", client.clientAddress))
				msg.WriteString(fmt.Sprintf("Server Address: %q\n", client.serverAddress))
				msg.WriteString(fmt.Sprintf("Query: %q\n", string(line)))
				msg.WriteString("\n\n")
				logger.Enqueue(msg.String())
			}
		}
	}()

	// send back to the client
	for {
		select {
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
			done <- true
			return
		}
	}

	// client.conn.Write([]byte("Message received.\n"))
}
