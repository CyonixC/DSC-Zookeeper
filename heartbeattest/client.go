package main

import (
	"bufio"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"os"
	"strings"
	"time"
)

var connectedServer string

// Main entry for client
func ClientMain() {
	recv, failedSendsChan := connectionManager.Init()

	//Listener
	go clientListener(recv)

	//Look for available servers
	findLiveServers()

	//Heartbeat
	go clientHeartbeat()

	for failedSends := range failedSendsChan {
		logger.Info(fmt.Sprint("Receive failedsend: ", failedSends))
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		command := strings.TrimSpace(scanner.Text())
		switch command {
		case "":
			continue

		default:
			fmt.Printf("Unknown command: %s\n", command)
			helpMessages()
		}
	}
}

func helpMessages() {
	fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
}

func clientListener(recv_channel chan connectionManager.NetworkMessage) {
	for network_msg := range recv_channel {
		logger.Info(fmt.Sprint("Receive message from ", network_msg.Remote))
	}
}

func findLiveServers() (bool) {
	for server := range config.Servers {
		data := []byte("HELLO")
		err := connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: config.Servers[server], Message: data})
		if err != nil {
			logger.Error(fmt.Sprint("Unable to connect to server: ", config.Servers[server]))
		} else {
			logger.Info(fmt.Sprint("Connected to server: ", config.Servers[server]))
			connectedServer = config.Servers[server]
			return true
		}
	}
	logger.Error("Unable to connect to any server")
	return false
}

func clientHeartbeat() {
	for {
		time.Sleep(time.Second * time.Duration(2))
		data := []byte("HEARTBEAT")
		err := connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: connectedServer, Message: data})
		if err != nil {
			logger.Error("Heartbeat failed to send: " + err.Error())
			for {
				logger.Info("Attempting to reconnect to a new server")
				success := findLiveServers()
				if success{
					break
				}
			}
		} else {
			logger.Info("Heartbeat sent to server: " + connectedServer)
		}
	}
}
