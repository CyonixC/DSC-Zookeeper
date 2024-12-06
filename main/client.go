package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"os"
	"strconv"
	"strings"
)

var connectedServer string
var sessionID string
var exist bool
var data string

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
		parts := strings.Split(command, " ")
		switch parts[0] {
		case "startsession":
			//Look for available servers and connect to one
			findLiveServer()

		case "endsession":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			msg := map[string]interface{}{
				"message":    "END_SESSION",
				"session_id": sessionID,
			}
			SendJSONMessage(msg, connectedServer)
		case "sync":
			if connectedServer == "" {
				logger.Error(fmt.Sprint("There is no session ongoing to sync"))
				continue
			}
			msg := map[string]interface{}{
				"message":    "SYNC",
				"session_id": sessionID,
			}
			SendJSONMessage(msg, connectedServer)
		case "create":
			if connectedServer == "" {
				logger.Error("There is no session ongoing to create")
				continue
			}

			if len(parts) != 3 {
				logger.Error("Missing path and data for 'create' command")
				continue
			}

			path := strings.TrimSpace(parts[1])
			data := strings.TrimSpace(parts[2])

			msg := map[string]interface{}{
				"message":    "CREATE",
				"session_id": sessionID,
				"path":       path,
				"data":       data,
			}
			SendJSONMessage(msg, connectedServer)
		case "delete":
			if connectedServer == "" {
				logger.Error("There is no session ongoing to delete")
				continue
			}
			input := strings.TrimSpace(scanner.Text())
			pathParts := strings.SplitN(input, " ", 2)
			if len(pathParts) < 2 || strings.TrimSpace(pathParts[1]) == "" {
				logger.Error("Error: Missing path and version for 'delete' command")
				continue
			}

			params := strings.SplitN(pathParts[1], " ", 2)
			if len(params) < 2 {
				logger.Error("Error: Missing version for 'delete' command")
				continue
			}
			path := strings.TrimSpace(params[0])
			version := strings.TrimSpace(params[1])

			msg := map[string]interface{}{
				"message":    "DELETE",
				"session_id": sessionID,
				"path":       path,
				"version":    version,
			}
			SendJSONMessage(msg, connectedServer)
		case "setdata":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			input := strings.TrimSpace(scanner.Text())
			pathParts := strings.SplitN(input, " ", 2)
			if len(pathParts) < 2 || strings.TrimSpace(pathParts[1]) == "" {
				logger.Error("Error: Missing path, data, and version for 'setdata' command")
				continue
			}
			params := strings.Fields(pathParts[1])
			if len(params) < 3 {
				logger.Error("Error: Missing data or version for 'setdata' command")
				continue
			}
			path := strings.TrimSpace(params[0])
			data := []byte(strings.TrimSpace(params[1]))
			versionStr := strings.TrimSpace(params[2])

			version, err := strconv.Atoi(versionStr)
			if err != nil {
				logger.Error(fmt.Sprintf("Error: Invalid version number '%s'", versionStr))
				continue
			}
			msg := map[string]interface{}{
				"message": "SETDATA",
				"path":    path,
				"data":    data,
				"version": version,
			}
			SendJSONMessage(msg, connectedServer)
		case "getchildren":
			if connectedServer == "" {
				logger.Error(fmt.Sprint("There is no session"))
				continue
			}
			input := strings.TrimSpace(scanner.Text())
			pathParts := strings.SplitN(input, " ", 2)
			if len(pathParts) < 2 || strings.TrimSpace(pathParts[1]) == "" {
				logger.Error("Error: Missing path for 'getchildren' command")
				continue
			}
			path := strings.TrimSpace(pathParts[1])
			msg := map[string]interface{}{
				"message": "GETCHILDREN",
				"path":    path,
			}
			SendJSONMessage(msg, connectedServer)
		case "exists":
			if connectedServer == "" {
				logger.Error(fmt.Sprint("There is no session"))
				continue
			}
			input := strings.TrimSpace(scanner.Text())
			pathParts := strings.SplitN(input, " ", 2)
			if len(pathParts) < 2 || strings.TrimSpace(pathParts[1]) == "" {
				logger.Error("Error: Missing path for 'exists' command")
				continue
			}
			path := strings.TrimSpace(pathParts[1])
			msg := map[string]interface{}{
				"message": "EXISTS",
				"path":    path,
			}
			SendJSONMessage(msg, connectedServer)
		case "getdata":
			if connectedServer == "" {
				logger.Error(fmt.Sprint("There is no session"))
				continue
			}
			input := strings.TrimSpace(scanner.Text())
			pathParts := strings.SplitN(input, " ", 2)
			if len(pathParts) < 2 || strings.TrimSpace(pathParts[1]) == "" {
				logger.Error("Error: Missing path for 'getdata' command")
				continue
			}
			path := strings.TrimSpace(pathParts[1])
			msg := map[string]interface{}{
				"message": "GETDATA",
				"path":    path,
			}
			SendJSONMessage(msg, connectedServer)
		case "publish": // this is the read, dont write data to znode, znode tells you who to write just for coordiantion
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}

		case "subscribe": //write, dont write data znode, write configuration, config as in like who u subscirbe to which topic
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			// add yourself to the topic znode though, from the name of the subszcribe
			// subscribe function,

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

	err = connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: server, Type: connectionManager.CLIENTMSG, Message: byteData})
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
	for network_msg := range recv_channel { //
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
		case "GETCHILDREN":
			children, err := obj["children"].([]string)
			if err {
				logger.Error("Invalid type for children: expected []string")
				continue
			}
			logger.Info(fmt.Sprint("Children is ", children))
		case "EXISTS":
			exist = obj["exists"].(bool)
			logger.Info(fmt.Sprint("It ", exist, "Exists"))
		case "GETDATA":
			data = obj["getdata"].(string)
			logger.Info(fmt.Sprint("The data is ", data))

		}
	}
}

// Loops through the list of servers to find one that is live.
// Connect to a server. Provide session ID if one is stored.
// If it is rejected, send another request with no session ID.
// If no session ID is stored, store the new ID.
func findLiveServer() bool {
	servers := config.Servers

	//For debugging: reverse the list of servers so that client 1 connects to the leader
	if os.Getenv("NAME") == "client1" {
		reversed := make([]string, len(servers))
		for i := 0; i < len(servers); i++ {
			reversed[i] = servers[len(servers)-i-1]
		}
		servers = reversed
	}

	for server := range servers {
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
