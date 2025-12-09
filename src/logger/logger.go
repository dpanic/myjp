// Package logger provides logging functionality for myjp using the zap logger.
// It includes both structured logging and a file-based query logging queue.
package logger

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log is the global zap logger instance used throughout the application.
	Log *zap.Logger
	// Debug controls whether debug logging is enabled (set via DEBUG environment variable).
	Debug = os.Getenv("DEBUG")
)

func init() {
	Log, _ = Setup(true)
	defer Log.Sync()
}

// Setup creates and configures a new zap logger instance.
// If verbose is true, the logger will output debug-level messages.
// Returns the configured logger and any error encountered during setup.
func Setup(verbose bool) (logger *zap.Logger, err error) {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}

	config := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          "console",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:    "message",
			LevelKey:      "level",
			TimeKey:       "time",
			NameKey:       "logger",
			CallerKey:     "go",
			StacktraceKey: "trace",
			LineEnding:    "\n",
			EncodeLevel:   zapcore.CapitalColorLevelEncoder,
			// EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
				now := time.Now()
				out := now.Format("02.01.2006 15:04:05.99")

				out = fmt.Sprintf("[ %s ]", out)
				enc.AppendString(out)
			},
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller: func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
				callerName := caller.TrimmedPath()
				callerName = minWidth(callerName, " ", 20)
				enc.AppendString(callerName)
			},
			EncodeName: zapcore.FullNameEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: nil,
		InitialFields:    nil,
	}

	return config.Build()
}

const (
	// fileLoc is the path to the query log file.
	fileLoc = "logs/queries.log"
)

var (
	// queue is a buffered channel for asynchronous log message writing.
	queue = make(chan string, 1024)
)

func init() {
	go func() {
		for {
			flushQueue()
			time.Sleep(3 * time.Second)
		}
	}()
}

// Enqueue adds a message to the logging queue for asynchronous writing to disk.
// If the queue is full, it triggers an immediate flush before adding the message.
func Enqueue(msg string) {
	select {
	case queue <- msg:
	case <-time.After(1 * time.Millisecond):
		flushQueue()
		queue <- msg
	}
}

// flushQueue writes all queued messages to the log file.
// It opens the file in append mode, writes all pending messages, and closes the file.
func flushQueue() {
	f, err := os.OpenFile(fileLoc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	for len(queue) > 0 {
		msg := <-queue
		f.WriteString(msg + "\n")
	}
}

// File writes data to a file at the specified location.
// It opens the file in append mode, creating it if it doesn't exist.
func File(fileLoc string, data string) (err error) {
	f, err := os.OpenFile(fileLoc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.WriteString(data)
	return
}
