package main

import (
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log/slog"
	"strconv"
	"time"
)

// Stress test for Docker + ConnectionManager
var prev []string

func main() {
	handler := logger.NewColouredTextHandler(slog.LevelInfo)
	logger.InitLogger(slog.New(handler))
	prev = make([]string, len(configReader.GetConfig().Servers))

	recv, failed := connectionManager.Init()
	time.Sleep(time.Second)
	go func() {
		for f := range failed {
			logger.Fatal(fmt.Sprint("Failed to send to ", f))
		}
	}()
	go func() {
		for r := range recv {
			time.Sleep(time.Millisecond)
			remoteId, _ := strconv.Atoi(string(r.Remote[len(r.Remote)-1]))
			logger.Info(fmt.Sprint("Received: ", string(r.Message), " from ", string(r.Remote), "; previous: ", prev[remoteId-1]))
			prev[remoteId-1] = string(r.Message)
		}
	}()

	for i := 0; ; i++ {
		time.Sleep(time.Microsecond * 200)
		connectionManager.Broadcast([]byte(fmt.Sprint("Testing ", i)))
	}
}
