package main

import (
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log/slog"
	"time"
)

// Stress test for Docker + ConnectionManager

func main() {
	handler := logger.NewColouredTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))

	recv, failed := connectionManager.Init()
	go func() {
		for f := range failed {
			logger.Fatal(fmt.Sprint("Failed to send to ", f))
		}
	}()
	go func() {
		for r := range recv {
			logger.Info(fmt.Sprint("Received: ", string(r.Message)))
		}
	}()

	for i := 0; ; i++ {
		time.Sleep(time.Second)
		connectionManager.Broadcast([]byte(fmt.Sprint("Testing ", i)))
	}
}
