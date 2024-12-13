package main

import (
	configReader "local/zookeeper/internal/ConfigReader"
	"local/zookeeper/internal/logger"
	"log/slog"
	"os"
	"time"
)

var config configReader.Config

// True main entry that calls either ServerMain or ClientMain depending on configuration.
func main() {
	mode := os.Getenv("MODE") // "Server" or "Client"
	handler := logger.NewColouredTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))

	config = *configReader.GetConfig()

	if mode == "Server" {
		logger.Info("Server starting...")
		go ServerMain()
	} else {
		logger.Info("Client starting...")
		go ClientMain()
	}
	time.Sleep(time.Hour)
}
