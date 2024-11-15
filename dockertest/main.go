package main

import (
	"encoding/json"
	"fmt"
	"io"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log"
	"log/slog"
	"os"
	"time"
)

type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}

func main() {
	handler := logger.NewPlainTextHandler(slog.LevelInfo)
	logger.InitLogger(slog.New(handler))
	recv, _ := connectionManager.Init()
	mode := os.Getenv("MODE") // "Server" or "Client"

	if mode == "Server" {
		go func() {
			for msg := range recv {
				logger.Info(fmt.Sprint("Message from ", msg.Remote, ": ", string(msg.Message)))
			}
		}()
		logger.Info("Server starting...")
		for {
			time.Sleep(time.Second * time.Duration(3))
			data := []byte("hello world 1")
			connectionManager.ServerBroadcast(data)
		}
	} else {
		logger.Info("Client starting...")
		for {
			time.Sleep(time.Second * time.Duration(5))
			data := []byte("hello world 2")
			connectionManager.Broadcast(data)
		}
	}
}

func loadConfig(filename string) Config {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	return config
}
