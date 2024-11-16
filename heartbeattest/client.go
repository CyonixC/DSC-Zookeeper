package main

import (
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log/slog"
	"os"
	"time"
	"bufio"
	"strings"
)

//Main entry for client
func ClientMain() {
	handler := logger.NewPlainTextHandler(slog.LevelInfo)
	logger.InitLogger(slog.New(handler))
	recv, _ := connectionManager.Init()

	//Listener
	go clientlistener(recv)

	//Heartbeat
	go clientHeartbeat()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		command := strings.TrimSpace(scanner.Text())
		if command == "" {
			continue
		}

		switch command {

		case "exit":
			fmt.Println("Exiting program.")
			return

		default:
			fmt.Printf("Unknown command: %s\n", command)
			clienthelp()
		}
	}
}

func clienthelp() {
	fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
}

// Listen for messages
func clientlistener(recv_channel chan connectionManager.NetworkMessage) {
	for network_msg := range recv_channel {
		fmt.Printf("Receive message from %s\n", network_msg.Remote)
	}
}

func clientHeartbeat(){
	for {
		time.Sleep(time.Second * time.Duration(5))
		data := []byte("HEARTBEAT")
		connectionManager.Broadcast(data)
	}
}