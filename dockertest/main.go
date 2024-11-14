package main

import (
	"encoding/json"
	"fmt"
	"io"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log"
	"os"
	"time"
)

type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}

func main() {
	mode := os.Getenv("MODE") // "Server" or "Client"

	if mode == "Server" {
		server_name := os.Getenv("NAME")
		startServer(server_name)
	} else {
		config := loadConfig("config.json")
		client_name := os.Getenv("NAME")
		startClient(client_name, config)
	}
}

func startServer(server_name string) {
	logger.Info(fmt.Sprint("Starting server ", server_name))
	recv := connectionManager.Init()
	for {
		msg := <-recv
		logger.Info(fmt.Sprint("Recv ", string(msg.Message), " from ", msg.Remote))
		time.Sleep(time.Second)
	}
}

func startClient(client_name string, config Config) {
	logger.Info(fmt.Sprint("Starting server ", client_name))
	for {
		for _, server := range config.Servers {
			data := []byte("hello world" + client_name)
			remoteAddr := server
			err := connectionManager.SendMessage(connectionManager.NetworkMessage{remoteAddr, data})
			if err != nil {
				logger.Info("Error: " + err.Error())
			} else {
				logger.Info(fmt.Sprint("Sent hello to ", remoteAddr))
			}
		}
		time.Sleep(time.Second * 5)
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
