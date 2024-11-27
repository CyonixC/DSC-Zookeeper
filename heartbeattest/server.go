package main

import (
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	//"time"
	"fmt"
	"encoding/json"
	"strings"
	"local/zookeeper/internal/znode"
	"local/zookeeper/internal/Proposals"
	// "math/rand"
	// "strconv"
)

var pending_requests []int

// Main entry for server
func ServerMain() {
	recv, _ := connectionManager.Init()
	_, _ = proposals.Init(recv, znode.Check)

	//Listener
	go clientListener(recv)
	// go committedListener(committed)
	// go deniedListener(denied)
	
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

// Listen for messages from client
func clientListener(recv_channel chan connectionManager.NetworkMessage) {
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
			obj := message.(map[string]interface{})
			switch obj["message"] {
			case "START_SESSION":
				// var new_session_id string
				// //generate random id and check if it does not exist
				// for { 
				// 	new_session_id := strconv.Itoa(rand.Intn(10000000))
				// 	if !znode.Exists_session(new_session_id) { break }
				// }
				// data, _ := znode.Encode_create_session(new_session_id, 2)

				// new_req_id := rand.Intn(10000000) 
				// //proposals.SendWriteRequest(data, new_req_id)
				// pending_requests = append(pending_requests, new_req_id)

			case "REESTABLISH_SESSION":
				// Check if session ID exist, return success if it is, else return failure with new ID.
			}
		}
	}
}

// func committedListener(committed_channel chan []byte) {
// 	for msg := range committed_channel {

// 	}
// }

// func deniedListener(denied_channel chan proposals.Request) {
// 	for msg := range denied_channel {

// 	}
// }

// Heartbeat the leader every x seconds
func serverHeartbeat() {
	// for {
	// 	time.Sleep(time.Second * time.Duration(3))
	// 	data := []byte("HEARTBEAT")
	// 	connectionManager.ServerBroadcast(data)
	// }
}
