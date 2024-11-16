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

// Main entry for client
func ClientMain() {
	recv, _ := connectionManager.Init()

	//Listener
	go clientListener(recv)

	//Heartbeat
	go clientHeartbeat()

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

func clientHeartbeat() {
	for {
		time.Sleep(time.Second * time.Duration(2))
		data := []byte("HEARTBEAT")
		err := connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: "server3", Message: data})
		if err != nil {
			logger.Error("Heartbeat failed to send: " + err.Error())
		} else {
			logger.Info("Heartbeat sent to server")
		}
	}
}
