package main

import (
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log/slog"
	"time"
)

// Main entry for server
func ServerMain() {
	handler := logger.NewPlainTextHandler(slog.LevelInfo)
	logger.InitLogger(slog.New(handler))
	recv, _ := connectionManager.Init()

	//Listener
	go serverlistener(recv)

	//Heartbeat
	go serverHeartbeat()
}

func serverhelp() {
	fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
}

// Listen for messages
func serverlistener(recv_channel chan connectionManager.NetworkMessage) {
	for network_msg := range recv_channel {
		fmt.Printf("Receive message from %s\n", network_msg.Remote)
	}
}

func serverHeartbeat() {
	for {
		time.Sleep(time.Second * time.Duration(3))
		data := []byte("HEARTBEAT")
		connectionManager.ServerBroadcast(data)
	}
}
