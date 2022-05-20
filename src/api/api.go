package api

import (
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
