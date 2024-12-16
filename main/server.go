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
var request_id_to_pending_request map[int]PendingRequest
var reqid_to_pendreq_mu sync.Mutex

type PendingRequest struct {
	is_from_client      bool                             // Whether the request is associated with a client request. Is true if and only if original_client_msg is not empty.
	original_client_msg connectionManager.NetworkMessage // Original client's message associated with the request. Might be empty.
	request             []byte                           // The request itself (the one sent to the proposal)
}

// Map clients connected to this server to session IDs.
// Add to this map when receiving a START_SESSION or REESTABLISH_SESSION, remove from this map on END_SESSION or when detect TCP closed.
var local_sessions map[string]string

type SessionInfo struct {
	SessionIds []string `json:"session_ids"`
}

var session_id_to_timestamp map[string]time.Time
var sessid_to_ts_mu sync.Mutex

// Time we give the client to reconnect to another server if its current server crashed.
const reconnect_timeout time.Duration = 10 * time.Second

// Main entry for server
func ServerMain() {
	request_id_to_pending_request = make((map[int]PendingRequest))
	local_sessions = make((map[string]string))

	recv, failedSends := connectionManager.Init()
	go monitorConnectionToClient(failedSends)
	go monitorConnectionToServer(failedSends)
	committed, denied, counter := proposals.Init(znode.Check)

	//Start election
	proposals.Pause()
	election.ElectionInit(counter)
	election.InitiateRingEntry()

	//init watch cache
	znode.Init_watch_cache()

	//Listeners
	go mainListener(recv)
	go committedListener(committed)
	go deniedListener(denied)
	go syncListener()

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			if configReader.GetName() == election.Coordinator.GetCoordinator() { // If this server is the coordinator
				removeExpiredSessions()
			} else {
				sendSessionInfo()
			}
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
		return nil
	}
}

// Helper function to send info/error messages back to client (i.e. just need to print in client terminal)
func SendInfoMessageToClient(info string, client string) {
	reply_msg := map[string]interface{}{
		"message": "INFO",
		"info":    info,
	}
	SendJSONMessageToClient(reply_msg, client)
}

// Listen for network messages from client or server
// Call different actions based on origin, msg type and msg header
func mainListener(recv_channel chan connectionManager.NetworkMessage) {
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
				session_id_to_timestamp = nil // Reset the map

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
				generateAndSendRequest(true, data, network_msg)

			case "REESTABLISH_SESSION":
				// Check if session ID exist, return success if it is, else return failure with new ID.
				if znode.Exists_session(obj["session_id"].(string)) {
					local_sessions[this_client] = obj["session_id"].(string)
					reply_msg := map[string]interface{}{
						"message":    "REESTABLISH_SESSION_OK",
						"session_id": obj["session_id"],
					}
					SendJSONMessageToClient(reply_msg, this_client)
					//Update watch cache
					request, paths, err := znode.Update_watch_cache(obj["session_id"].(string))
					if err != nil {
						logger.Error(fmt.Sprint("Error in updating watch cache:", err))
					}
					//check if any flags triggered during reconnect
					if len(request) > 0 {
						generateAndSendRequest(false, request, connectionManager.NetworkMessage{})
						for _, path := range paths {
							watch_msg := map[string]interface{}{
								"message": "WATCH_TRIGGER",
								"path":    path,
							}
							SendJSONMessageToClient(watch_msg, this_client)
						}
					}
				} else {
					reply_msg := map[string]interface{}{
						"message": "REESTABLISH_SESSION_REJECT",
					}
					SendJSONMessageToClient(reply_msg, this_client)
				}

			case "END_SESSION":
				data, err := znode.Encode_delete_session(obj["session_id"].(string))
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}

				logger.Info(fmt.Sprint("Sending session end request: ", obj["session_id"].(string), " to leader"))
				generateAndSendRequest(true, data, network_msg)

			case "PUBLISH":
				//If topic doesn't exist, create it.
				//Else create a new message

				exists := znode.Exists(obj["topic"].(string))
				if !exists { //Create the topic if it doesn't exist
					data, _ := znode.Encode_create(obj["topic"].(string), []byte{}, false, false, obj["session_id"].(string))
					generateAndSendRequest(true, data, network_msg)
				} else { //Else, create message with seq flag
					path := obj["topic"].(string) + "/msg"
					data, _ := znode.Encode_create(path, []byte(obj["data"].(string)), false, true, obj["session_id"].(string))
					generateAndSendRequest(true, data, network_msg)
				}

			case "SUBSCRIBE":
				//If topic doesn't exist, create it.
				//Else subscribe to the topic

				exists := znode.Exists(obj["topic"].(string))
				if !exists { //Create the topic if it doesn't exist
					data, _ := znode.Encode_create(obj["topic"].(string), []byte{}, false, false, obj["session_id"].(string))

					obj["newtopic"] = true //Append a flag to the message so that we know we still need to add the watch flag later
					modifiedMessage, _ := json.Marshal(obj)
					network_msg.Message = modifiedMessage
					generateAndSendRequest(true, data, network_msg)
				} else { //Else, watch the next msg in that topic (highest message number +1, currently nonexistant)
					children, _ := znode.GetChildren(obj["topic"].(string))
					nextmsgnum := findMaxMessageNumber(children) + 1
					path := obj["topic"].(string) + "/msg_" + strconv.Itoa(nextmsgnum)
					logger.Debug("Creating watch flag on " + path)
					data, _ := znode.Encode_watch(obj["session_id"].(string), path, true)

					obj["newtopic"] = false
					modifiedMessage, _ := json.Marshal(obj)
					network_msg.Message = modifiedMessage
					generateAndSendRequest(true, data, network_msg)
				}

			case "GET_SUBSCRIPTION":
				//Send all unseen messages to the client
				//Then add a new watch flag for the next message that doesn't exist yet

				topic := obj["topic"].(string)
				nextmsgnum := int(obj["nextMsg"].(float64))
				for {
					path := topic + "/msg_" + strconv.Itoa(nextmsgnum)
					if znode.Exists(path) {
						z, _ := znode.GetData(path)
						data := string(z.Data)
						msg := map[string]interface{}{
							"message": "SUBSCRIBED_MESSAGE",
							"topic":   topic,
							"data":    data,
						}
						SendJSONMessageToClient(msg, this_client)
						nextmsgnum++
					} else {
						reply_msg := map[string]interface{}{
							"message": "GET_SUBSCRIPTION_OK",
							"topic":   obj["topic"].(string),
							"nextMsg": nextmsgnum,
						}
						SendJSONMessageToClient(reply_msg, this_client)
						break
					}
				}

			/// EXTRA COMMANDS: for testing & demonstration of zookeeper
			case "SYNC":
				data, err := znode.Encode_sync()
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info(fmt.Sprint("Sending sync request", obj["session_id"].(string), "to leader"))
				generateAndSendRequest(true, data, network_msg)
			case "CREATE":
				data := []byte(obj["data"].(string))
				ephemeralStr := obj["ephemeral"].(string)
				ephemeral, _ := strconv.ParseBool(ephemeralStr)
				sequentialStr := obj["sequential"].(string)
				sequential, _ := strconv.ParseBool(sequentialStr)
				request, err := znode.Encode_create(obj["path"].(string), data, ephemeral, sequential, obj["session_id"].(string)) // where i get the sequential and ephermeral from
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				generateAndSendRequest(true, request, network_msg)
			case "DELETE":
				versionFloat := obj["version"].(float64)
				version := int(versionFloat)
				data, err := znode.Encode_delete(obj["path"].(string), version)
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info("Sending delete request")
				generateAndSendRequest(true, data, network_msg)
			case "SETDATA":
				data := []byte(obj["data"].(string))
				versionFloat := obj["version"].(float64)
				version := int(versionFloat)
				request, err := znode.Encode_setdata(obj["path"].(string), data, version)
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info("Setting data")
				generateAndSendRequest(true, request, network_msg)
			case "GETCHILDREN":
				children, err := znode.GetChildren(obj["path"].(string))
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				logger.Info("Getting children")
				watchStr := obj["watch"].(string)
				watch, err := strconv.ParseBool(watchStr)
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				if watch {
					request, err := znode.Encode_watch(local_sessions[this_client], obj["path"].(string), true)
					if err != nil {
						SendInfoMessageToClient(err.Error(), this_client)
					}
					logger.Info("Adding watch flag")
					generateAndSendRequest(true, request, network_msg)
				}
				reply_msg := map[string]interface{}{
					"message":  "GETCHILDREN_OK",
					"children": children,
				}
				SendJSONMessageToClient(reply_msg, this_client)
			case "EXISTS":
				path := obj["path"].(string)
				exists := znode.Exists(path)
				watchStr := obj["watch"].(string)
				watch, err := strconv.ParseBool(watchStr)
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				if watch {
					request, err := znode.Encode_watch(local_sessions[this_client], path, true)
					if err != nil {
						SendInfoMessageToClient(err.Error(), this_client)
					}
					logger.Info("Adding watch flag")
					generateAndSendRequest(true, request, network_msg)
				}
				reply_msg := map[string]interface{}{
					"message": "EXISTS_OK",
					"exists":  exists,
					"path":    path,
				}
				SendJSONMessageToClient(reply_msg, this_client)
			case "GETDATA":
				if configReader.GetConfig().TestMode == "mode2" && configReader.GetName() == "server1" {
					logger.Fatal("Mode2: Server1 panics just after receiving a SET_DATA request (before it sends any proposals)")
					panic("Mode2")
				}
				znode_data, err := znode.GetData(obj["path"].(string))
				if err != nil {
					logger.Error("There is error in getdata")
				}
				watchStr := obj["watch"].(string)
				watch, err := strconv.ParseBool(watchStr)
				if err != nil {
					SendInfoMessageToClient(err.Error(), this_client)
				}
				if watch {
					request, err := znode.Encode_watch(local_sessions[this_client], obj["path"].(string), true)
					if err != nil {
						SendInfoMessageToClient(err.Error(), this_client)
					}
					logger.Info("Adding watch flag")
					generateAndSendRequest(true, request, network_msg)
				}
				reply_msg := map[string]interface{}{
					"message": "GETDATA_OK",
					"znode":   znode_data,
				}
				SendJSONMessageToClient(reply_msg, this_client)
			}

			//Handle ZAB messages from other server
		} else if network_msg.Type == connectionManager.ZAB {
			//Handle messages from other server through proposals package
			proposals.ProcessZabMessage(network_msg)
		} else if network_msg.Type == connectionManager.HEARTBEAT {
			//Handle heartbeats
			recvSessionInfo(network_msg.Message)
		}
	}
}

// Listener for proposal commit messages
// Reply to the client with an OK
func committedListener(committed_channel chan proposals.Request) {
	for request := range committed_channel {
		logger.Info(fmt.Sprint("Receive commit ", request.ReqNumber, ": Type ", request.ReqType))
		modified_paths, err := znode.Write(request.Content)
		if err != nil {
			logger.Error(fmt.Sprint("Error when attempting to commit: ", err.Error()))
			return
		}

		//Check for triggered watch flags.
		//If any connected client is watching, inform the client of the change
		//Then send a write request to remove the watch flag
		data, sessions, _ := znode.Check_watch(modified_paths)
		if len(sessions) > 0 {
			// for each session with triggered watch flags
			for _, session := range sessions {
				// get client for each session id
				for client, session_id := range local_sessions {
					if session_id == session {
						// for each (modified) path that triggered a watch flag, send a message to the client
						for _, path := range modified_paths {
							watch_msg := map[string]interface{}{
								"message": "WATCH_TRIGGER",
								"path":    path,
							}
							SendJSONMessageToClient(watch_msg, client)
						}
						break
					}
				}
			}
			generateAndSendRequest(false, data, connectionManager.NetworkMessage{})
		}

		pending_request, exists := request_id_to_pending_request[request.ReqNumber]

		//If it's my client, remove from pending_request map and reply to client
		if exists && pending_request.is_from_client {
			original_message := pending_request.original_client_msg

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

			case "PUBLISH":
				//If the message has been published, send OK to client
				//If just the topic has been created, generate new proposal to publish the message now.

				children, _ := znode.GetChildren(obj["topic"].(string))
				if len(children) == 0 { //Topic was just created, no messages created yet
					//Create msg and send a new request
					path := obj["topic"].(string) + "/msg"
					data, _ := znode.Encode_create(path, []byte(obj["data"].(string)), false, true, obj["session_id"].(string))
					generateAndSendRequest(true, data, original_message)
					reply_msg = map[string]interface{}{
						"message": "CREATE_TOPIC_OK",
					}
				} else { //Message successfully added
					reply_msg = map[string]interface{}{
						"message": "PUBLISH_OK",
					}
				}

			case "SUBSCRIBE":
				//If the topic has been subscribed, send OK to client
				//If just the topic has been created, generate new proposal to subscribe to the topic now.

				if obj["newtopic"].(bool) { //Topic was just created, watch flag not set yet
					children, _ := znode.GetChildren(obj["topic"].(string))
					nextmsgnum := findMaxMessageNumber(children) + 1
					path := obj["topic"].(string) + "/msg_" + strconv.Itoa(nextmsgnum)
					logger.Debug("Creating watch flag on " + path)
					data, _ := znode.Encode_watch(obj["session_id"].(string), path, true)

					obj["newtopic"] = false
					modifiedMessage, _ := json.Marshal(obj)
					original_message.Message = modifiedMessage
					generateAndSendRequest(true, data, original_message)

					reply_msg = map[string]interface{}{
						"message": "CREATE_TOPIC_OK",
					}
				} else {
					children, _ := znode.GetChildren(obj["topic"].(string))
					nextmsgnum := findMaxMessageNumber(children) + 1
					reply_msg = map[string]interface{}{
						"message": "SUBSCRIBE_OK",
						"topic":   obj["topic"].(string),
						"nextMsg": nextmsgnum,
					}
				}

			case "GET_SUBSCRIPTION":
				children, _ := znode.GetChildren(obj["topic"].(string))
				nextmsgnum := findMaxMessageNumber(children) + 1
				reply_msg = map[string]interface{}{
					"message": "GET_SUBSCRIPTION_OK",
					"topic":   obj["topic"].(string),
					"nextMsg": nextmsgnum,
				}

			/// EXTRA COMMANDS: for testing & demonstration of zookeeper
			case "CREATE":
				if configReader.GetConfig().TestMode == "mode5" && configReader.GetName() == "server1" {
					logger.Fatal("Mode5 Server1 panics after receiving commit, but before responding to client (create request)")
					panic("Mode5")
				}
				reply_msg = map[string]interface{}{
					"message": "CREATE_OK",
					"path":    obj["path"],
				}
			case "DELETE":
				reply_msg = map[string]interface{}{
					"message": "DELETE_OK",
					"path":    obj["path"],
				}
			case "SETDATA":
				reply_msg = map[string]interface{}{
					"message": "SETDATA_OK",
					"path":    obj["path"],
				}
			case "SYNC":
				reply_msg = map[string]interface{}{
					"message": "SYNC_OK",
					"path":    obj["path"],
				}
			default:
				continue
			}

			SendJSONMessageToClient(reply_msg, original_message.Remote)

			if configReader.GetConfig().TestMode == "mode1" && configReader.GetName() == "server1" {
				logger.Fatal("Mode1: Server1 panics just after admitting a client's startsession request")
				panic("Mode1")
			}
		}

		delete(request_id_to_pending_request, request.ReqNumber)
	}
}

// Listener for proposal denied messages
func deniedListener(denied_channel chan proposals.Request) {
	for request := range denied_channel {
		logger.Info(fmt.Sprint("Receive denied ", request.ReqNumber, ": Type ", request.ReqType))

		pending_request, exists := request_id_to_pending_request[request.ReqNumber]

		if !exists {
			continue
		}

		//If it's my client, remove from pending_request map and reply to client
		if pending_request.is_from_client {
			original_message := pending_request.original_client_msg

			var message interface{}
			json.Unmarshal([]byte(original_message.Message), &message)
			obj := message.(map[string]interface{})
			reply_msg := map[string]interface{}{
				"message": "REJECT",
			}
			switch obj["message"] {
			case "START_SESSION":
				reply_msg = map[string]interface{}{
					"message": "START_SESSION_REJECT",
				}

				SendJSONMessageToClient(reply_msg, original_message.Remote)
				delete(request_id_to_pending_request, request.ReqNumber)
			//read ops only send a proposal when watch is true, hence if failed, failure to propogate watch flag
			//BUT will successfully add to local watch cache
			//Instead of replying to client, retry propogaing watch flag
			case "GETCHILDREN", "GETDATA", "EXISTS":
				logger.Info("Retrying watch flag propagation")
				//check local watch cache to ensure watch flag has not been triggered yet
				//if triggered, do not retry
				// TODO may want to add a sleep? or smth similar to prevent spamming of this
				sessionids := znode.Get_watching_sessions(obj["path"].(string))
				for _, sessionid := range sessionids {
					if sessionid == local_sessions[original_message.Remote] {
						data, _ := znode.Encode_watch(local_sessions[original_message.Remote], obj["path"].(string), false)
						generateAndSendRequest(true, data, original_message)
						break
					}
				}
			default:
				SendJSONMessageToClient(reply_msg, original_message.Remote)
				delete(request_id_to_pending_request, request.ReqNumber)
			}
		}
		// TODO: Else case. What happens if non-client proposals are denied? (Can it even happen?)
		// Eg: proposal to delete ephemeral nodes

	}
}

func syncListener() {
	for range proposals.SyncFinish {
		logger.Info("Sync finish detected.")

		if election.Coordinator.GetCoordinator() == configReader.GetName() {
			logger.Info("Coordinator re-initializing session id map")
			initializeSessionIdMap()
		}

		// Resend all pending write requests
		reqid_to_pendreq_mu.Lock()
		logger.Info("Resending all pending requests")
		for request_id, pending_request := range request_id_to_pending_request {
			proposals.SendWriteRequest(pending_request.request, request_id)
		}
		reqid_to_pendreq_mu.Unlock()
	}

}

// Helper function to generate unique request ID, send request, and append the new request to pending_request map
func generateAndSendRequest(is_from_client bool, data []byte, original_message connectionManager.NetworkMessage) {
	new_req_id := generateUniqueRequestID()
	proposals.SendWriteRequest(data, new_req_id)

	reqid_to_pendreq_mu.Lock()
	request_id_to_pending_request[new_req_id] = PendingRequest{is_from_client: is_from_client, original_client_msg: original_message, request: data}
	reqid_to_pendreq_mu.Unlock()
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

		_, exists := request_id_to_pending_request[new_req_id]

		if !exists {
			return new_req_id
		}
	}
}

// Monitor TCP connection for failures
func monitorConnectionToClient(failedSends chan string) {
	for failedNode := range failedSends {
		//If it's my client, remove from local_sessions
		session_id, exists := local_sessions[failedNode]
		if exists {
			request, err := znode.Encode_delete_session(session_id)
			if err != nil {
				logger.Error(fmt.Sprint("Error in deleting session:", err))
			}

			generateAndSendRequest(false, request, connectionManager.NetworkMessage{})
			logger.Info(fmt.Sprint("Session deleting a timed-out session_id", session_id))

			delete(local_sessions, failedNode)
		}
	}
}

// If connection to another server fails, trigger an election
func monitorConnectionToServer(failedSends chan string) {
	for failedNode := range failedSends {
		if election.Coordinator.GetCoordinator() == failedNode {
			election.InitiateElectionDiscovery()
		} else {
			ring_structure := election.ReorderRing(configReader.GetConfig().Servers, configReader.GetName())
			messageWrapper := election.MessageWrapper{
				Message_Type: 2,
				Source:       configReader.GetName(),
			}
			updatedRing := election.HandleNewRingMessage(ring_structure, messageWrapper, configReader.GetName())
			election.Addresses = updatedRing
			logger.Info(fmt.Sprint("Updated ring structure for node ", configReader.GetName(), updatedRing))
		}
	}
}

// Send a heartbeat every 5 seconds to tell the leader all the clients connected to this server
func sendSessionInfo() {
	var session_ids []string
	for _, session_id := range local_sessions {
		session_ids = append(session_ids, session_id)
	}

	session_info_json, err := json.Marshal(SessionInfo{
		SessionIds: session_ids,
	})

	if err != nil {
		logger.Error(fmt.Sprint("Failed to encode json session info", err))
	}

	var network_msg connectionManager.NetworkMessage
	network_msg.Type = connectionManager.HEARTBEAT
	network_msg.Remote = election.Coordinator.GetCoordinator()
	network_msg.Message = session_info_json

	logger.Debug(fmt.Sprint("Sending list of connection session_ids", session_ids))
	err = connectionManager.SendMessage(network_msg)
	if err != nil {
		logger.Error(fmt.Sprint("Failed to send heartbeat: ", err))
		election.InitiateElectionDiscovery()
	}
}

// For coordinator. Creates a map of session:timers to track timeouts of all the sessions
func initializeSessionIdMap() {
	session_id_to_timestamp = make(map[string]time.Time)
	session_ids, err := znode.Get_sessions()
	if err != nil {
		logger.Error(fmt.Sprint("Failed to get sessions from znode", err))
	}

	sessid_to_ts_mu.Lock()
	for _, session_id := range session_ids {
		session_id_to_timestamp[session_id] = time.Now()
	}
	sessid_to_ts_mu.Unlock()
}

// For coordinator. Reset the session timeout when received a heartbeat from server.
func recvSessionInfo(session_info_json []byte) {
	var session_info SessionInfo
	err := json.Unmarshal(session_info_json, &session_info)

	if err != nil {
		logger.Error(fmt.Sprint("Failed to decode json session info", err))
	}

	// If we received session info before sync finished, ignore it.
	if session_id_to_timestamp == nil {
		return
	}

	sessid_to_ts_mu.Lock()
	for _, session_id := range session_info.SessionIds {
		session_id_to_timestamp[session_id] = time.Now()
	}
	sessid_to_ts_mu.Unlock()
}

// For coordinator. Remove expired sessions when they timeout.
func removeExpiredSessions() {
	if session_id_to_timestamp == nil {
		return
	}

	curr_time := time.Now()

	sessid_to_ts_mu.Lock()
	for session_id, timestamp := range session_id_to_timestamp {
		if curr_time.Sub(timestamp) < reconnect_timeout {
			continue
		}

		request, err := znode.Encode_delete_session(session_id)

		if err != nil {
			logger.Error(fmt.Sprint("Error in deleting session:", err))
		}

		generateAndSendRequest(false, request, connectionManager.NetworkMessage{})

		logger.Info(fmt.Sprint("Coordinator deleting a timed-out session_id", session_id))
		delete(session_id_to_timestamp, session_id)
	}
	sessid_to_ts_mu.Unlock()
}

// Helper func for finding the max message number from a list of znodes
// If empty, return 0
func findMaxMessageNumber(messages []string) int {
	maxNum := 0
	for _, msg := range messages {
		// Strip the "msg" prefix and convert to an integer
		numStr := strings.TrimPrefix(msg, "msg_")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue // Skip if there's an error in conversion
		}
		// Update the max number
		if num > maxNum {
			maxNum = num
		}
	}
	return maxNum
}
