package api

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/dpanic/myjp/src/config"
	"github.com/dpanic/myjp/src/logger"
	"go.uber.org/zap"
)

func Run() {
	configs, err := config.LoadAll()

	if err != nil {
		logger.Log.Panic("error in reading configuration",
			zap.Error(err),
		)
	}

	for _, config := range configs {
		server := NewServer(config)
		go server.Run()
	}

	select {}
}

// genID generates random ID
func genID() string {
	raw := fmt.Sprintf("%v", time.Now().UnixNano())

	h := sha256.New()
	h.Write([]byte(raw))
	bs := string(h.Sum(nil))

	return fmt.Sprintf("%x", bs[0:7])
}
