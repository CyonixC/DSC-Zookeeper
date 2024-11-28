package election

import (
	"encoding/json"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	proposals "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/logger"
	"slices"
	"sync"
)

type MessageType int

var addresses []string = configReader.GetConfig().Servers
var mu sync.Mutex
var Coordinator string
const (
	MessageTypeDiscovery = iota
	MessageTypeAnnouncement
	MessageTypeNewRing
	MessageTypeHeartbeat
)

type MessageWrapper struct {
	Message_Type  MessageType
	Source        string
	Visited_Nodes []string
	ZxId_List []uint32
	Payload       []string
}

func ReorderRing(ring_structure []string, id string) []string {
	startIndex := slices.Index(ring_structure, id)
	if startIndex == -1 {
		fmt.Println("Error: id not found in ring_structure")
		return ring_structure 
	}
	afterStart := ring_structure[startIndex+1:]
	beforeStart := ring_structure[:startIndex]
	reorderedRing := slices.Concat(afterStart, beforeStart)
	return reorderedRing
}

// dont think need just broadcast
func SendRingAnnouncement(ring []string, content []string, messageType MessageType) {
	StartRingMessage(ring, content, messageType)
}
func StartRingMessage( ring_structure []string, messageContent []string, messageType MessageType) {
	nodeIP:= configReader.GetName()
	visitedNodes := make([]string, 0, len(ring_structure))
	visitedNodes = append(visitedNodes, nodeIP)
	zxidList := make([]uint32, 0, len(ring_structure))
	zxidList = append(zxidList, proposals.ZxidCounter.GetLatestZXID())
	message := MessageWrapper{
		messageType,
		nodeIP,
		visitedNodes,
		zxidList,
		messageContent,
	}
	go DispatchMessage(ring_structure, message)
}

   
func Pass_message_down_ring(ring_structure []string, message MessageWrapper) bool {
	id := configReader.GetName()
	if slices.Contains(message.Visited_Nodes, id) {
		logger.Info(fmt.Sprintf("Node %s already visited, completing ring pass.", id))
		return true
	} else {
		message.Visited_Nodes = append(message.Visited_Nodes, id)
		message.ZxId_List = append(message.ZxId_List, proposals.ZxidCounter.GetLatestZXID())
		logger.Info(fmt.Sprintf("Updated Visited Nodes: %s, thread id: %s\n", message.Visited_Nodes, id))
		logger.Info(fmt.Sprintf("Updated Zxid: [%d], thread id: %s\n", message.ZxId_List, id))

		message.Source = id
		go DispatchMessage(ring_structure, message)
		return false
	}
}
func DispatchMessage(ring_structure []string, message_cont MessageWrapper) {
	messageBytes, err := json.Marshal(message_cont)
	if err != nil {
		logger.Info(fmt.Sprint("Error encoding message:", err))
		return
	}
loop:
	for _, target := range ring_structure {
		err := connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: target, Message: messageBytes})
		if err != nil {
			logger.Info(fmt.Sprintf("Failed to send message to %v; retrying with next target", target))
			fmt.Printf("Error: %v\n", err)
			continue
		} else {
			logger.Info(fmt.Sprintf("Message sent successfully to %v", target))
			break loop
		}
	}
}
func getCorrespondingValue(numArray []uint32, charArray []string) string {
	if len(numArray) == 0 || len(numArray) != len(charArray) {
		panic("Arrays must be non-empty and of the same length")
	}

	maxIndex := 0
	for i := 1; i < len(numArray); i++ {
		if numArray[i] > numArray[maxIndex] || 
		   (numArray[i] == numArray[maxIndex] && charArray[i] > charArray[maxIndex]) {
			maxIndex = i
		}
	}

	return charArray[maxIndex]
}

func HandleDiscoveryMessage(ring_structure []string, message MessageWrapper, failechan chan string) {
	nodeIP := configReader.GetName()
	isComplete := Pass_message_down_ring(ring_structure, message)
	if isComplete {
		logger.Info(fmt.Sprintf("Completed Discovery in %s: %v", nodeIP, message.Visited_Nodes))
		if len(message.Visited_Nodes) >= len(addresses)/2{
			electedCoordinator := getCorrespondingValue(message.ZxId_List, message.Visited_Nodes)
			logger.Info(fmt.Sprintf("New Coordinator: %v", electedCoordinator))
			ring_struct := ReorderRing(message.Visited_Nodes, nodeIP)
			go SendRingAnnouncement(ring_struct, []string{electedCoordinator}, MessageTypeAnnouncement)
		} else{
			logger.Error(fmt.Sprint("No enough majority votes dk why teehee"))
		}

	}
}
func InitiateElectionDiscovery() {
	nodeIP := configReader.GetName()
	initRing := ReorderRing(addresses, nodeIP)
	logger.Info(fmt.Sprintf("Node %s initiated election discovery\n", nodeIP))
	initialContent := []string{}
	StartRingMessage(initRing, initialContent, MessageTypeDiscovery)

}
// Processes an announcement message and initiates a new ring announcement if the election finishes.
func HandleAnnouncementMessage(ring []string, message MessageWrapper, failedchan chan string) string {
	isComplete := Pass_message_down_ring(ring, message)
	if isComplete {
		logger.Info(fmt.Sprintf("Finish Election"))
		go SendRingAnnouncement(ring, message.Visited_Nodes, MessageTypeNewRing)
	}
	return slices.Max(message.Payload)
}

func HandleNewRingMessage(ring_structure []string, message MessageWrapper, id string, failedchan chan string) []string {
	Pass_message_down_ring(ring_structure, message)
	return message.Payload
}
func HandleMessage(nodeIP string, failedChan chan string, messageWrapper MessageWrapper) bool {
	
	switch messageWrapper.Message_Type {
	case MessageTypeDiscovery:
		ring_structure := ReorderRing(addresses, nodeIP)
		HandleDiscoveryMessage(ring_structure, messageWrapper, failedChan)
		return true
	case MessageTypeAnnouncement:
		ring_structure := ReorderRing(addresses, nodeIP)
		Coordinator = HandleAnnouncementMessage(ring_structure, messageWrapper, failedChan)
		logger.Info(fmt.Sprint(nodeIP, " acknowledges new coordinator ", Coordinator))
		return false
	case MessageTypeNewRing:
		ring_structure := ReorderRing(addresses, nodeIP)
		updatedRing := HandleNewRingMessage(ring_structure, messageWrapper, nodeIP, failedChan)
		addresses = updatedRing
		logger.Info(fmt.Sprint("Updated ring structure for node ", nodeIP, updatedRing))
		return false
	default:
		return false
	}
}

func ElectionInit() (){
	addresses = configReader.GetConfig().Servers
}

