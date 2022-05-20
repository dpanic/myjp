package logger

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log   *zap.Logger
	Debug = os.Getenv("DEBUG")
)

func init() {
	Log, _ = Setup(true)
	defer Log.Sync()
}

// configure will return instance of zap logger configuration, configured to be verbose or to use JSON formatting
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
	fileLoc = "logs/queries.log"
)

var (
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

// Enqueue sets message for writing to disk
func Enqueue(msg string) {
	select {
	case queue <- msg:
	case <-time.After(1 * time.Millisecond):
		flushQueue()
		queue <- msg
	}
}

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

func File(fileLoc string, data string) (err error) {
	f, err := os.OpenFile(fileLoc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.WriteString(data)
	return
}
