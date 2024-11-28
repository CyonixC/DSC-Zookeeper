package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"os"
	"strings"
)

var connectedServer string
var sessionID string

// Main entry for client
func ClientMain() {
	recv, failedSends := connectionManager.Init()
	go monitorConnectionToServer(failedSends)

	//Main Listener
	go listener(recv)

	//Main loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		command := strings.TrimSpace(scanner.Text())
		switch command {
		case "startsession":
			//Look for available servers and connect to one
			findLiveServer()

		case "endsession":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			msg := map[string]interface{}{
				"message": "END_SESSION",
				"session_id": sessionID,
			}
			SendJSONMessage(msg, connectedServer)

		case "publish":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}

		case "subscribe":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}

		default:
			fmt.Printf("Unknown command: %s\n", command)
			fmt.Println("Available commands: startsession, endsession, publish, subscribe")
		}
	}
}

// Send unstrctured data to the connected server
func SendJSONMessage(jsonData interface{}, server string) error {
	logger.Info(fmt.Sprint("Sending message: ", jsonData))
	// Convert to JSON ([]byte)
	byteData, err := json.Marshal(jsonData)
	if err != nil {
		fmt.Println("Error serializing to JSON:", err)
		return err
	}

	err = connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: server, Message: byteData})
	if err != nil {
		logger.Error("Error sending message to server, please try again.")
		return err
	} else {
		logger.Info("Message send success.")
	}
	return nil
}

// Main listener for all messages set from server
func listener(recv_channel chan connectionManager.NetworkMessage) {
	for network_msg := range recv_channel {
		logger.Info(fmt.Sprint("Receive message from ", network_msg.Remote))
		var message interface{}
		err := json.Unmarshal([]byte(network_msg.Message), &message)
		if err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}
		logger.Info(fmt.Sprint("Map Data: ", message))

		// Type assertion to work with the data
		obj := message.(map[string]interface{})
		switch obj["message"] {
		case "INFO":
			fmt.Println(obj["info"])
		case "START_SESSION_OK":
			// Store the new ID
			sessionID = obj["session_id"].(string)
			fmt.Println("Session established successfully.")
		case "REESTABLISH_SESSION_OK":
			fmt.Println("Session reestablished successfully.")
		case "REESTABLISH_SESSION_REJECT":
			sessionID = ""
			fmt.Println("Session has expired, please startsession again.")
		case "END_SESSION_OK":
			sessionID = ""
			fmt.Println("Session ended successfully.")
		}
	}
}

// Loops through the list of servers to find one that is live.
// Connect to a server. Provide session ID if one is stored.
// If it is rejected, send another request with no session ID.
// If no session ID is stored, store the new ID.
func findLiveServer() bool {
	for server := range config.Servers {
		var msg interface{}
		if sessionID == "" {
			msg = map[string]interface{}{
				"message": "START_SESSION",
			}
		} else {
			msg = map[string]interface{}{
				"message":    "REESTABLISH_SESSION",
				"session_id": sessionID,
			}
		}
		err := SendJSONMessage(msg, config.Servers[server])
		if err != nil {
			logger.Error(fmt.Sprint("Unable to connect to server: ", config.Servers[server]))
		} else {
			logger.Info(fmt.Sprint("Found a server: ", config.Servers[server]))
			connectedServer = config.Servers[server]
			return true
		}
	}
	logger.Error("Unable to connect to any server")
	return false
}

// Monitor TCP connection
func monitorConnectionToServer(failedSends chan string) {
	for failedNode := range failedSends {
		if failedNode == connectedServer {
			logger.Error(fmt.Sprint("TCP connection to connected server failed: ", failedNode))
			connectedServer = ""
			logger.Info("Attempting to reconnect to a new server")
			findLiveServer()
		}
	}
}
