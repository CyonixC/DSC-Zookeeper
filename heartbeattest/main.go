package main

import (
	"encoding/json"
	"io"
	"local/zookeeper/internal/logger"
	"log/slog"
	"os"
	"fmt"
)

type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}

var config Config

func main() {
	mode := os.Getenv("MODE") // "Server" or "Client"
	handler := logger.NewPlainTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))

	config = loadConfig("./config.json")
 
	if mode == "Server" {
		logger.Info("Server starting...")
		go ServerMain()
	} else {
		logger.Info("Client starting...")
		go ClientMain()
	}

	select{}
}

func loadConfig(filename string) Config {
	file, err := os.Open(filename)
	if err != nil {
		logger.Fatal(fmt.Sprint("Failed to open config file: ", err))
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		logger.Fatal(fmt.Sprint("Failed to read config file: ", err))
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		logger.Fatal(fmt.Sprint("Failed to parse config file: ", err))
	}

	return config
}
