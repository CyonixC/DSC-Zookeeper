package main

import (
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	configReader "local/zookeeper/internal/ConfigReader"
	proposals "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/logger"
	"local/zookeeper/internal/znode"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var pending_requests map[int]connectionManager.NetworkMessage //Map request IDs to the original message sent by client

// Main entry for server
func ServerMain() {
	pending_requests = make((map[int]connectionManager.NetworkMessage))

	recv, _ := connectionManager.Init()
	committed, denied := proposals.Init(znode.Check)

	//TODO call election instead
	if configReader.GetName() == "server1" {
		znode.Init_znode_cache()
	}

	//Listeners
	go mainListener(recv)
	go committedListener(committed)
	go deniedListener(denied)

	//Heartbeat
	// go serverHeartbeat()
}

// Send unstrctured data to a client
func SendJSONMessageToClient(jsonData interface{}, client string) error {
	logger.Info(fmt.Sprint("Sending message: ", jsonData, " to ", client))
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
func mainListener(recv_channel chan connectionManager.NetworkMessage) {
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
				var new_session_id string
				//Generate random session id and check if it does not exist
				for {
					new_session_id = strconv.Itoa(rand.Intn(10000000))
					if !znode.Exists_session(new_session_id) {
						break
					}
				}

				logger.Info(fmt.Sprint("Sending session write request: ", new_session_id, " to leader"))
				data, _ := znode.Encode_create_session(new_session_id, 2)

				//Generate a request ID, send the request, and add it to pending_requests
				new_req_id := generateUniqueRequestID()
				proposals.SendWriteRequest(data, new_req_id)
				pending_requests[new_req_id] = network_msg

			case "REESTABLISH_SESSION":
				// Check if session ID exist, return success if it is, else return failure with new ID.
			}
		} else {
			//Handle messages from other server
			proposals.ProcessZabMessage(network_msg)
		}
	}
}

//Generate a random unique request ID
//Ensuring that different servers cannot generate the same ID by prepending with server number
func generateUniqueRequestID() int {
	server_name := configReader.GetName()
	lastDigit := rune(server_name[len(server_name)-1])
	server_number, _ := strconv.Atoi(string(lastDigit))

	var new_req_id int
	for {
		new_req_id = server_number*1000000000 + rand.Intn(10000000) 
		_, exists := pending_requests[new_req_id]
		if !exists {
			return new_req_id
		}
	}
}

func committedListener(committed_channel chan proposals.Request) {
	for request := range committed_channel {
		logger.Info(fmt.Sprint("Receive commit ", request.ReqNumber, ": Type ", request.ReqType))
		modified_paths, err := znode.Write(request.Content)
		if err != nil {
			logger.Error(fmt.Sprint("Error when attempting to commit: ", err.Error()))
			return
		}

		//If it's my client, remove from pending_requests and reply to client
		original_message, exists := pending_requests[request.ReqNumber]
		if exists {
			var message interface{}
			json.Unmarshal([]byte(original_message.Message), &message)
			var reply_msg interface{}
			obj := message.(map[string]interface{})
				switch obj["message"] {
				case "START_SESSION":
					//Get session ID from filepath
					segments := strings.Split(modified_paths[0], "/")
					session_id := segments[len(segments)-1]
					reply_msg = map[string]interface{}{
						"message": "START_SESSION_OK",
						"session_id": session_id,
					}
				}

			SendJSONMessageToClient(reply_msg, pending_requests[request.ReqNumber].Remote)
			delete(pending_requests, request.ReqNumber)
		}
	}
}

func deniedListener(denied_channel chan proposals.Request) {
	for request := range denied_channel {
		logger.Info(fmt.Sprint("Receive denied ", request.ReqNumber, ": Type ", request.ReqType))

		//If it's my client, remove from pending_requests and reply to client
		original_message, exists := pending_requests[request.ReqNumber]
		if exists {
			var message interface{}
			json.Unmarshal([]byte(original_message.Message), &message)
			var reply_msg interface{}
			obj := message.(map[string]interface{})
				switch obj["message"] {
				case "START_SESSION":
					reply_msg = map[string]interface{}{
						"message": "START_SESSION_REJECT",
					}
				}

			SendJSONMessageToClient(reply_msg, pending_requests[request.ReqNumber].Remote)
			delete(pending_requests, request.ReqNumber)
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
