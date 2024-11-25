package main

import (
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"time"
	"fmt"
	"encoding/json"
	"strings"
)

// Main entry for server
func ServerMain() {
	recv, _ := connectionManager.Init()

	//Listener
	go serverlistener(recv)

	//Heartbeat
	go serverHeartbeat()

	
}

// Send unstrctured data to a client
func SendJSONMessageToClient(jsonData interface{}, client string) error{
	logger.Info(fmt.Sprint("Sending message: ", jsonData , " to ", client))
	// Convert to JSON ([]byte)
	byteData, err := json.Marshal(jsonData)
	if err != nil {
		fmt.Println("Error serializing to JSON:", err)
		return err
	}

	err = connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: client, Message: byteData})
	if err != nil {
		logger.Error(fmt.Sprint("Error sending message: ", client))
		return err
	} else {
		logger.Info("Message send success.")
	}
	return nil
}

// Listen for messages
func serverlistener(recv_channel chan connectionManager.NetworkMessage) {
	for network_msg := range recv_channel {
		logger.Info(fmt.Sprint("Receive message from ", network_msg.Remote))

		//Handle messages from client
		if strings.HasPrefix(network_msg.Remote, "client") {
			var message interface{}
			err := json.Unmarshal([]byte(network_msg.Message), &message)
			if err != nil {
				fmt.Println("Error parsing JSON:", err)
				return
			}
			logger.Info(fmt.Sprint("Map Data: ", message))

			// Type assertion to work with the data
			items := message.([]interface{})
			for _, item := range items {
				obj := item.(map[string]interface{})
				switch obj["message"] {
				case "START_SESSION":
					// Return a new session ID
				case "REESTABLISH_SESSION":
					// Check if session ID exist, return success if it is, else return failure with new ID.
				}
			}
		}
	}
}

// Heartbeat the leader every x seconds
func serverHeartbeat() {
	for {
		time.Sleep(time.Second * time.Duration(3))
		data := []byte("HEARTBEAT")
		connectionManager.ServerBroadcast(data)
	}
}
