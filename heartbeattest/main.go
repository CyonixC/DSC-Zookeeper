package main

import (
	"encoding/json"
	"io"
	"local/zookeeper/internal/logger"
	"log"
	"os"
)

type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}

func main() {
	mode := os.Getenv("MODE") // "Server" or "Client"

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
