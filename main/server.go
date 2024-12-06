package main

import (
	"encoding/json"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	proposals "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/election"
	"local/zookeeper/internal/logger"
	"local/zookeeper/internal/znode"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Map request IDs to the original message sent by client
var pending_requests map[int]connectionManager.NetworkMessage

// Map clients connected to this server to session IDs.
// Add to this map when receiving a START_SESSION or REESTABLISH_SESSION, remove from this map on END_SESSION or when detect TCP closed.
var local_sessions map[string]string

// Main entry for server
func ServerMain() {
	pending_requests = make((map[int]connectionManager.NetworkMessage))
	local_sessions = make((map[string]string))

	recv, failedSends := connectionManager.Init()
	go monitorConnectionToClient(failedSends)
	committed, denied, counter := proposals.Init(znode.Check)

	//Start election
	proposals.Pause()
	election.ElectionInit(counter)
	election.InitiateElectionDiscovery()

	//Listeners
	go mainListener(recv)
	go committedListener(committed)
	go deniedListener(denied)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			// TODO: Fix this
			// If server is coordinator:
			// removeExpiredSessions()
			// Else:
			// sendSessionInfo()
		}
	}()

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

// Helper function to send info/error messages back to client (i.e. just need to print in client terminal)
func SendInfoMessageToClient(info string, client string) {
	reply_msg := map[string]interface{}{
		"message": "INFO",
		"info":    info,
	}
	SendJSONMessageToClient(reply_msg, client)
}

// Listen for messages
func mainListener(recv_channel chan connectionManager.NetworkMessage) {

	// election.HandleMessage(messageWrapper)
	for network_msg := range recv_channel {

		this_client := network_msg.Remote
		logger.Info(fmt.Sprint("Receive message from ", this_client))

		//Handle election messages
		if network_msg.Type == connectionManager.ELECTION {
			var messageWrapper election.MessageWrapper
			err := json.Unmarshal(network_msg.Message, &messageWrapper)
			if err != nil {
				logger.Fatal(fmt.Sprint("Error unmarshalling message:", err))
			}
			electionstatus := election.HandleMessage(messageWrapper)
			if !electionstatus {
				logger.Info("Election Completed")
				if election.Coordinator.GetCoordinator() == configReader.GetName() {
					znode.Init_znode_cache()
					proposals.Continue()
				}
			}

			//Handle messages from client
		} else if network_msg.Type == connectionManager.CLIENTMSG {
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
				data, _ := znode.Encode_create_session(new_session_id, 2)

				logger.Info(fmt.Sprint("Sending session write request: ", new_session_id, " to leader"))
				generateAndSendRequest(data, network_msg) //Generate a request ID, send the request, and add it to pending_requests

			case "REESTABLISH_SESSION":
				// Check if session ID exist, return success if it is, else return failure with new ID.
				if znode.Exists_session(obj["session_id"].(string)) {
					local_sessions[this_client] = obj["session_id"].(string)
					reply_msg := map[string]interface{}{
						"message":    "REESTABLISH_SESSION_OK",
						"session_id": obj["session_id"],
					}
					SendJSONMessageToClient(reply_msg, this_client)
				} else {
					reply_msg := map[string]interface{}{
						"message": "REESTABLISH_SESSION_REJECT",
					}
					SendJSONMessageToClient(reply_msg, this_client)
				}
				//TODO add znode.Update_watch_cache()

			case "END_SESSION":
				data, err := znode.Encode_delete_session(obj["session_id"].(string))
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}

				logger.Info(fmt.Sprint("Sending session end request: ", obj["session_id"].(string), " to leader"))
				generateAndSendRequest(data, network_msg)
			case "SYNC":
				data, err := znode.Encode_sync()
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info(fmt.Sprint("Sending sync request", obj["session_id"].(string), "to leader"))
				generateAndSendRequest(data, network_msg)
			case "CREATE":
				data, err := znode.Encode_create(obj["path"].(string), obj["data"].([]byte), true, true, obj["session_id"].(string)) // where i get the sequential and ephermeral from
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info(fmt.Sprint("Sending create request", obj["data"].([]byte), " to leader"))
				generateAndSendRequest(data, network_msg)
			case "DELETE":
				data, err := znode.Encode_delete(obj["path"].(string), obj["version"].((int)))
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info(fmt.Sprint("Sending delete request"))
				generateAndSendRequest(data, network_msg)
			case "SETDATA":
				data, err := znode.Encode_setdata(obj["path"].(string), obj["data"].([]byte), obj["version"].(int))
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info(fmt.Sprint("Setting data"))
				generateAndSendRequest(data, network_msg)
			case "GETCHILDREN":
				children, err := znode.GetChildren(obj["path"].(string))
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info(fmt.Sprint("Getting children"))
				reply_msg := map[string]interface{}{
					"message":  "GETCHILDREN",
					"children": children,
				}
				SendJSONMessageToClient(reply_msg, this_client)
			case "EXISTS":
				exists := znode.Exists(obj["path"].(string))
				reply_msg := map[string]interface{}{
					"message": "EXISTS",
					"exists":  exists,
				}
				SendJSONMessageToClient(reply_msg, this_client)
			case "GETDATA":
				getdata, err := znode.GetData(obj["path"].(string))
				if err != nil {
					logger.Error(fmt.Sprint("There is error in getdata"))
				}
				reply_msg := map[string]interface{}{
					"message": "GETDATA",
					"getdata": getdata,
				}
				SendJSONMessageToClient(reply_msg, this_client)
			}

			//Handle ZAB messages from other server
		} else if network_msg.Type == connectionManager.ZAB {
			//Handle messages from other server
			proposals.ProcessZabMessage(network_msg)
		} else if network_msg.Type == connectionManager.HEARTBEAT {
			//Handle heartbeats
			recvSessionInfo(network_msg.Message)
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
					"message":    "START_SESSION_OK",
					"session_id": session_id,
				}
				local_sessions[original_message.Remote] = session_id

			case "END_SESSION":
				reply_msg = map[string]interface{}{
					"message": "END_SESSION_OK",
				}
				delete(local_sessions, original_message.Remote)
			}

			SendJSONMessageToClient(reply_msg, original_message.Remote)
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

// Helper function to generate unique request ID, send request, and append the new request to pending_requests
func generateAndSendRequest(data []byte, original_message connectionManager.NetworkMessage) {
	new_req_id := generateUniqueRequestID()
	proposals.SendWriteRequest(data, new_req_id)
	pending_requests[new_req_id] = original_message
}

// Generate a random unique request ID
// Ensuring that different servers cannot generate the same ID by prepending with server number
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

// Monitor TCP connection
func monitorConnectionToClient(failedSends chan string) {
	for failedNode := range failedSends {
		//If it's my client, remove from local_sessions
		_, exists := local_sessions[failedNode]
		if exists {
			delete(local_sessions, failedNode)
		} else {
			election.InitiateElectionDiscovery() //! Why can client connections trigger elections
		}
	}
}

type SessionInfo struct {
	SessionIds []string `json:"session_ids"`
}

var session_id_to_timestamp = make(map[string]time.Time)
var map_mu sync.Mutex

const timeout time.Duration = 10 * time.Second

// For servers. Send this every 5 seconds or something
func sendSessionInfo() {
	// TODO: Get all session_ids connected to this server
	var session_ids []string = []string{"id1", "id2"}

	session_info_json, err := json.Marshal(SessionInfo{
		SessionIds: session_ids,
	})

	if err != nil {
		logger.Error("Failed to encode json session info", err)
	}

	var network_msg connectionManager.NetworkMessage
	network_msg.Type = connectionManager.HEARTBEAT
	network_msg.Remote = election.Coordinator.GetCoordinator()
	network_msg.Message = session_info_json

	logger.Debug("Sending list of connection session_ids", session_ids)
	err = connectionManager.SendMessage(network_msg)
	if err != nil {
		logger.Error("Failed to send message", err)
	}
}

// For leader. Run it once when a server gets elected as leader
func initializeMap() {
	session_ids, err := znode.Get_sessions()
	if err != nil {
		logger.Error("Failed to get sessions from znode", err)
	}

	map_mu.Lock()
	for _, session_id := range session_ids {
		session_id_to_timestamp[session_id] = time.Now()
	}
	map_mu.Unlock()
}

// For leader
func recvSessionInfo(session_info_json []byte) {
	var session_info SessionInfo
	err := json.Unmarshal(session_info_json, &session_info)

	if err != nil {
		logger.Error("Failed to decode json session info", err)
	}

	map_mu.Lock()
	for _, session_id := range session_info.SessionIds {
		session_id_to_timestamp[session_id] = time.Now()
	}
	map_mu.Unlock()

}

// For leader. Run this every 5 seconds or something
func removeExpiredSessions() {
	curr_time := time.Now()

	map_mu.Lock()
	for session_id, timestamp := range session_id_to_timestamp {
		if curr_time.Sub(timestamp) < timeout {
			continue
		}

		request, err := znode.Encode_delete_session(session_id)

		if err != nil {
			logger.Error("Error in deleting session:", err)
		}

		proposals.SendWriteRequest(request, generateUniqueRequestID())

		logger.Debug("Deleting a timed-out session_id", session_id)
		delete(session_id_to_timestamp, session_id)
	}
	map_mu.Unlock()
}
