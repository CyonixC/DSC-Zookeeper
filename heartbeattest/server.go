package main

import (
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"time"
	"fmt"
)

// Main entry for server
func ServerMain() {
	recv, _ := connectionManager.Init()

	//Listener
	go serverlistener(recv)

	//Heartbeat
	go serverHeartbeat()
}

// Listen for messages
func serverlistener(recv_channel chan connectionManager.NetworkMessage) {
	for network_msg := range recv_channel {
		logger.Info(fmt.Sprint("Receive message from ", network_msg.Remote))
	}
}

func serverHeartbeat() {
	for {
		time.Sleep(time.Second * time.Duration(3))
		data := []byte("HEARTBEAT")
		connectionManager.ServerBroadcast(data)
	}
}
