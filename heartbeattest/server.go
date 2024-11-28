package main

import (
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	proposals "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/logger"
	"local/zookeeper/internal/znode"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var pending_requests []int

// Main entry for server
func ServerMain() {
	time.Sleep(time.Second)
	recv, _ := connectionManager.Init()
	time.Sleep(time.Second)
	
	committed, denied := proposals.Init(znode.Check)

	//Listener
	go clientListener(recv)
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
				var new_session_id string
				//generate random id and check if it does not exist
				for {
					new_session_id = strconv.Itoa(rand.Intn(10000000))
					if !znode.Exists_session(new_session_id) {
						break
					}
				}

				logger.Info(fmt.Sprint("Sending session write request: ", new_session_id, " to leader"))
				data, _ := znode.Encode_create_session(new_session_id, 2)
				new_req_id := rand.Intn(10000000)
				proposals.SendWriteRequest(data, new_req_id)
				pending_requests = append(pending_requests, new_req_id)

			case "REESTABLISH_SESSION":
				// Check if session ID exist, return success if it is, else return failure with new ID.
			}
		} else {
			//Handle messages from other server
			proposals.ProcessZabMessage(network_msg)
		}
	}
}

func committedListener(committed_channel chan proposals.Request) {
	for _ = range committed_channel {
		logger.Info(fmt.Sprint("Receive commit message"))
	}
}

func deniedListener(denied_channel chan proposals.Request) {
	for _ = range denied_channel {
		logger.Info(fmt.Sprint("Receive denied message"))
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
