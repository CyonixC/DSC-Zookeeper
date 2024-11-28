package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"os"
	"strings"
	"time"
)

var connectedServer string
var sessionID int

// Main entry for client
func ClientMain() {
	recv, _ := connectionManager.Init()
	sessionID = -1 //Initialize as -1 representing no session ID

	//Main Listener
	go listener(recv)

	//Look for available servers
	findLiveServer()

	//Start Heartbeat
	// go heartbeat()

	//Main loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		command := strings.TrimSpace(scanner.Text())
		switch command {
		case "":
			continue

		default:
			fmt.Printf("Unknown command: %s\n", command)
			fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
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
		case "START_SESSION_OK":
			// Store the new ID
		case "REESTABLISH_SESSION_OK":
			// OK
		case "REESTABLISH_SESSION_REJECT":
			// Store the new ID
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
		if sessionID == -1 {
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

// Heartbeat the server every x seconds and attempts reconnect if message fails to send
// TODO replace with monitoring TCP connection
func heartbeat() {
	for {
		time.Sleep(time.Second * time.Duration(2))
		var msg interface{} = map[string]interface{}{
			"message": "HEARTBEAT",
		}
		if connectedServer != "" {
			err := SendJSONMessage(msg, connectedServer)
			if err != nil {
				logger.Error("Heartbeat failed to send: " + err.Error())
				connectedServer = ""
				for {
					logger.Info("Attempting to reconnect to a new server")
					success := findLiveServer() //Find a new server when heartbeat fails
					if success {
						break
					}
				}
			} else {
				logger.Info("Heartbeat sent to server: " + connectedServer)
			}
		}
	}
}
