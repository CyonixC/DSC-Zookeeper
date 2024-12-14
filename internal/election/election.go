package election

import (
	"encoding/json"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"slices"
	"sync"
)

type MessageType int

var Addresses []string = configReader.GetConfig().Servers
var mu sync.Mutex

const (
	MessageTypeDiscovery = iota
	MessageTypeAnnouncement
	MessageTypeNewRing
	MessageTypeHeartbeat
)

type CoordinatorStruct struct {
	sync.Mutex
	newCoordinator string
}

var Coordinator CoordinatorStruct

type MessageWrapper struct {
	Message_Type  MessageType
	Source        string
	Visited_Nodes []string
	ZxId_List     []uint32
	Payload       []string
}

type ZXIDRef interface {
	GetLatestZXID() uint32
}

var zxidCounter ZXIDRef

func (coordinator *CoordinatorStruct) setCoordinator(electedCoordinator string) {
	coordinator.Lock()
	defer coordinator.Unlock()
	coordinator.newCoordinator = electedCoordinator
}
func (coordinator *CoordinatorStruct) GetCoordinator() string {
	coordinator.Lock()
	defer coordinator.Unlock()
	return coordinator.newCoordinator
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
func StartRingMessage(ring_structure []string, messageContent []string, messageType MessageType) {
	nodeIP := configReader.GetName()
	visitedNodes := make([]string, 0, len(ring_structure))
	visitedNodes = append(visitedNodes, nodeIP)
	zxidList := make([]uint32, 0, len(ring_structure))
	zxidList = append(zxidList, zxidCounter.GetLatestZXID())
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
		message.ZxId_List = append(message.ZxId_List, zxidCounter.GetLatestZXID())
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
		err := connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: target, Type: connectionManager.ELECTION, Message: messageBytes})
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

func HandleDiscoveryMessage(ring_structure []string, message MessageWrapper) {
	nodeIP := configReader.GetName()
	isComplete := Pass_message_down_ring(ring_structure, message)
	if isComplete {
		logger.Info(fmt.Sprintf("Completed Discovery in %s: %v", nodeIP, message.Visited_Nodes))
		if len(message.Visited_Nodes) >= (len(Addresses)/2)+1 {
			electedCoordinator := getCorrespondingValue(message.ZxId_List, message.Visited_Nodes)
			logger.Info(fmt.Sprintf("New Coordinator: %v", electedCoordinator))
			Addresses = message.Visited_Nodes
			ring_struct := ReorderRing(message.Visited_Nodes, nodeIP)
			go SendRingAnnouncement(ring_struct, []string{electedCoordinator}, MessageTypeAnnouncement)
		} else {
			logger.Error(fmt.Sprint("No enough majority votes dk why teehee"))
		}

	}
}
func InitiateElectionDiscovery() {
	nodeIP := configReader.GetName()
	Addresses = configReader.GetConfig().Servers
	initRing := ReorderRing(Addresses, nodeIP)
	logger.Info(fmt.Sprintf("Node %s initiated election discovery\n", nodeIP))
	initialContent := []string{}
	StartRingMessage(initRing, initialContent, MessageTypeDiscovery)
}

// Processes an announcement message and initiates a new ring announcement if the election finishes.
func HandleAnnouncementMessage(ring []string, message MessageWrapper) string {
	isComplete := Pass_message_down_ring(ring, message)
	if isComplete {
		logger.Info(fmt.Sprintf("Finish Election"))
		go SendRingAnnouncement(ring, message.Visited_Nodes, MessageTypeNewRing)
	}
	return slices.Max(message.Payload)
}

func HandleNewRingMessage(ring_structure []string, message MessageWrapper, id string) []string {
	Pass_message_down_ring(ring_structure, message)
	return message.Payload
}
func HandleMessage(messageWrapper MessageWrapper) bool {
	nodeIP := configReader.GetName()
	switch messageWrapper.Message_Type {
	case MessageTypeDiscovery:
		ring_structure := ReorderRing(Addresses, nodeIP)
		HandleDiscoveryMessage(ring_structure, messageWrapper)
		return true
	case MessageTypeAnnouncement:
		ring_structure := ReorderRing(Addresses, nodeIP)
		newcoords := HandleAnnouncementMessage(ring_structure, messageWrapper)
		Coordinator.setCoordinator(newcoords)
		logger.Info(fmt.Sprint(nodeIP, " acknowledges new coordinator ", newcoords))
		return true
	case MessageTypeNewRing:
		ring_structure := ReorderRing(Addresses, nodeIP)
		updatedRing := HandleNewRingMessage(ring_structure, messageWrapper, nodeIP)
		Addresses = updatedRing
		logger.Info(fmt.Sprint("Updated ring structure for node ", nodeIP, updatedRing))
		return false
	default:
		return false
	}
}

func ElectionInit(counter ZXIDRef) {
	Addresses = configReader.GetConfig().Servers
	zxidCounter = counter
}
