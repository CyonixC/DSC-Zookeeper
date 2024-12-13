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
var topicMsgMap map[string]int //Map of subscribed topic: next message ID to be read

var exist bool
var versions map[string]int

// Main entry for client
func ClientMain() {
	recv, failedSends := connectionManager.Init()
	versions = make(map[string]int)
	topicMsgMap = make(map[string]int)

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
		//Connect to server and get a session ID
		case "startsession":
			//Look for available servers and connect to one
			findLiveServer()

		//Disconnect from server
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

		//Publish some data to a topic. Creates a topic if it does not exist.
		case "publish":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			if len(parts) != 3 {
				fmt.Println("Usage: publish <topic> <data>")
				continue
			}

			topic := strings.TrimSpace(parts[1])
			data := strings.TrimSpace(parts[2])

			msg := map[string]interface{}{
				"message":    "PUBLISH",
				"session_id": sessionID,
				"topic":      topic,
				"data":       data,
			}
			SendJSONMessage(msg, connectedServer)

		//Subscribe (watch) a topic
		case "subscribe":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			if len(parts) != 2 {
				fmt.Println("Usage: subscribe <topic>")
				continue
			}

			topic := strings.TrimSpace(parts[1])

			msg := map[string]interface{}{
				"message":    "SUBSCRIBE",
				"session_id": sessionID,
				"topic":      topic,
			}
			SendJSONMessage(msg, connectedServer)

		/// EXTRA COMMANDS: for testing & demonstration of zookeeper
		case "sync":
			if connectedServer == "" {
				logger.Error("There is no session ongoing to sync")
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

			if len(parts) != 5 {
				logger.Error("Missing path, data, ephemeral, sequential for 'create' command")
				continue
			}

			path := strings.TrimSpace(parts[1])
			data := strings.TrimSpace(parts[2])
			ephemeral := strings.TrimSpace(parts[3])
			sequential := strings.TrimSpace(parts[4])
			msg := map[string]interface{}{
				"message":    "CREATE",
				"session_id": sessionID,
				"path":       path,
				"data":       data,
				"ephemeral":  ephemeral,
				"sequential": sequential,
			}
			SendJSONMessage(msg, connectedServer)

		case "delete":
			if connectedServer == "" {
				logger.Error("There is no session ongoing to delete")
				continue
			}

			if len(parts) != 2 {
				logger.Error("Missing path for 'delete' command")
				continue
			}
			path := strings.TrimSpace(parts[1])
			msg := map[string]interface{}{
				"message":    "DELETE",
				"session_id": sessionID,
				"path":       path,
				"version":    versions[path],
			}
			SendJSONMessage(msg, connectedServer)

		case "setdata":
			if connectedServer == "" {
				fmt.Println("Error: Session has not started")
				continue
			}
			if len(parts) != 3 {
				logger.Error("Missing path and data for 'setdata' command")
				continue
			}
			path := strings.TrimSpace(parts[1])
			data := strings.TrimSpace(parts[2])
			msg := map[string]interface{}{
				"message": "SETDATA",
				"path":    path,
				"data":    data,
				"version": versions[path],
			}
			SendJSONMessage(msg, connectedServer)

		case "getchildren":
			if connectedServer == "" {
				logger.Error("There is no session")
				continue
			}
			if len(parts) != 3 {
				logger.Error("Missing path or watch flag for 'getchildren' command")
				continue
			}
			path := strings.TrimSpace(parts[1])
			watch := strings.TrimSpace(parts[2])
			msg := map[string]interface{}{
				"message": "GETCHILDREN",
				"path":    path,
				"watch":   watch,
			}
			SendJSONMessage(msg, connectedServer)

		case "exists":
			if connectedServer == "" {
				logger.Error("There is no session")
				continue
			}
			if len(parts) != 2 {
				logger.Error("Missing path or watch flag for 'exists' command")
				continue
			}
			path := strings.TrimSpace(parts[1])
			watch := strings.TrimSpace(parts[2])
			msg := map[string]interface{}{
				"message": "EXISTS",
				"path":    path,
				"watch":   watch,
			}
			SendJSONMessage(msg, connectedServer)

		case "getdata":
			if connectedServer == "" {
				logger.Error("There is no session")
				continue
			}
			if len(parts) != 2 {
				logger.Error("Missing path or watch flag for 'getdata' command")
				continue
			}
			path := strings.TrimSpace(parts[1])
			watch := strings.TrimSpace(parts[2])
			msg := map[string]interface{}{
				"message": "GETDATA",
				"path":    path,
				"watch":   watch,
			}
			SendJSONMessage(msg, connectedServer)

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

		obj := message.(map[string]interface{})
		switch obj["message"] {
		case "INFO":
			fmt.Println(obj["info"])
		case "START_SESSION_OK":
			// Store the new session ID
			sessionID = obj["session_id"].(string)
			fmt.Println("Session established successfully.")
		case "REESTABLISH_SESSION_OK":
			fmt.Println("Session reestablished successfully.")
		case "REESTABLISH_SESSION_REJECT":
			// Remove session ID
			sessionID = ""
			fmt.Println("Session has expired, please startsession again.")
		case "END_SESSION_OK":
			sessionID = ""
			fmt.Println("Session ended successfully.")
		case "CREATE_TOPIC_OK":
			fmt.Println("Topic created successfully.")
		case "PUBLISH_OK":
			fmt.Println("Message published successfully.")
		case "SUBSCRIBE_OK":
			fmt.Println("Successfully subscribed to topic:", obj["topic"].(string))
			topicMsgMap[obj["topic"].(string)] = int(obj["nextMsg"].(float64))
		case "WATCH_TRIGGER":
			fmt.Println("Watched path has been modified:", obj["path"].(string))

			//If the path is a subscribed topic, request for all the newest messages
			topLevelPath := strings.Split(obj["path"].(string), "/")[0]
			nextMsgNum, topicExists := topicMsgMap[topLevelPath]
			if topicExists {
				msg := map[string]interface{}{
					"message":    "GET_SUBSCRIPTION",
					"topic":      topLevelPath,
					"nextMsg":    nextMsgNum,
					"session_id": sessionID,
				}
				SendJSONMessage(msg, connectedServer)
			}
		case "SUBSCRIBED_MESSAGE":
			fmt.Println("Received message from topic", obj["topic"].(string), ":", obj["data"].(string))
		case "GET_SUBSCRIPTION_OK":
			//Update the next message to watch for
			topicMsgMap[obj["topic"].(string)] = int(obj["nextMsg"].(float64))
			//Set watch flag for the topic
			path := obj["topic"].(string) + "/msg_" + strconv.Itoa(int(obj["nextMsg"].(float64)))
			msg := map[string]interface{}{
				"message": "EXISTS",
				"path":    path,
				"watch":   "true",
			}
			SendJSONMessage(msg, connectedServer)

		/// EXTRA COMMANDS: for testing & demonstration of zookeeper
		case "SYNC_OK":
			path := obj["path"].(string)
			versions[path]++
			fmt.Println("Sync Ok")
		case "CREATE_OK":
			path := obj["path"].(string)
			if _, exists := versions[path]; !exists {
				versions[path] = 1
			}
			fmt.Println("Create Ok")
		case "DELETE_OK":
			path := obj["path"].(string)
			delete(versions, path)
			fmt.Println("Delete Ok")
		case "SETDATA_OK":
			path := obj["path"].(string)
			versions[path]++
			fmt.Println("SetData Ok ", versions[path])
		case "GETCHILDREN_OK":
			children, err := obj["children"].([]string)
			if err {
				logger.Error("Invalid type for children: expected []string")
				continue
			}
			logger.Info(fmt.Sprint("Children is ", children))
		case "EXISTS_OK":
			exist = obj["exists"].(bool)
			logger.Info(fmt.Sprint("Exists = ", exist))
		case "GETDATA_OK":
			jsonData, err := json.MarshalIndent(obj["znode"], "", "  ")
			if err != nil {
				fmt.Println("Error marshalling to JSON:", err)
				return
			}
			logger.Info(fmt.Sprint(string(jsonData)))
			path := obj["path"].(string)
			fmt.Println("Watch flag triggered for path: ", path)
		case "WATCH_FAIL":
			logger.Error(fmt.Sprint("Watch flag was set but not propogated"))
		case "REJECT":
			logger.Error(fmt.Sprint("Some request has been rejected"))
		}
	}
}

// Loops through the list of servers to find one that is live.
// Connect to a server. Provide session ID if one is stored.
// If it is rejected, send another request with no session ID.
// If no session ID is stored, store the new ID.
func findLiveServer() bool {
	servers := config.Servers

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
