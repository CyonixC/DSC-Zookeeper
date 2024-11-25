package election

import (
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"slices"
	"sync"
)
type MessageType int
var addresses = []string{"server1", "server2", "server3", "server4"}
var mu sync.Mutex

const (
	MessageTypeDiscovery = iota
	MessageTypeAnnouncement
	MessageTypeNewRing
	MessageTypeHeartbeat
)

type MessageWrapper struct {
	Message_Type MessageType
	Source         string
	Visited_Nodes  []string
    Payload        []string
}

func ReorderRing(ring_structure []string, id string) []string {
    fmt.Printf("ring_structure: %v, id: %s\n", ring_structure, id)

    startIndex := slices.Index(ring_structure, id)
    if startIndex == -1 {
        fmt.Println("Error: id not found in ring_structure")
        return ring_structure // Return as-is if the ID is not found
    }
    fmt.Printf("startIndex: %d\n", startIndex)
    afterStart := ring_structure[startIndex+1:]
    fmt.Printf("afterStart: %v\n", afterStart)
    beforeStart := ring_structure[:startIndex]
    fmt.Printf("beforeStart: %v\n", beforeStart)
    reorderedRing := slices.Concat(afterStart, beforeStart)
    fmt.Printf("reorderedRing: %v\n", reorderedRing)
    return reorderedRing
}

// dont think need just broadcast
func SendRingAnnouncement(nodeIP string, ring []string, content []string, messageType MessageType, failedchan chan string ) {
    StartRingMessage(nodeIP, ring, content, messageType, failedchan)
}
func StartRingMessage(nodeIP string, ring_structure []string, messageContent []string, messageType MessageType, failedchan chan string) {
    // 2,3,4
    visitedNodes := make([]string, 0, len(ring_structure))
    visitedNodes = append(visitedNodes, nodeIP)
    fmt.Printf("Start Ring Message %s\n", nodeIP)
    message := MessageWrapper{
        messageType,
        nodeIP,
        visitedNodes,
		messageContent,
    }
    fmt.Printf("Start Ring Message Ring sturct %s\n", ring_structure)

    go DispatchMessage(ring_structure,message,failedchan)
}


func InitiateElectionDiscovery(nodeIP string,  failedchan chan string) {
    fmt.Print("addressess %v", addresses)
    initRing:=ReorderRing(addresses,nodeIP)
    logger.Info(fmt.Sprintf("Node %s initiated election discovery\n", nodeIP))
    initialContent := []string{}
    StartRingMessage(nodeIP, initRing, initialContent, MessageTypeDiscovery, failedchan)
	
}
func Pass_message_down_ring(ring_structure []string, message MessageWrapper, id string, failedchan chan string) bool {
    fmt.Print("Passed down the message ring struct",message.Visited_Nodes)
    if slices.Contains(message.Visited_Nodes, id) {
        logger.Info(fmt.Sprintf("Node %s already visited, completing ring pass.\n", id))
        return true 
    } else {
        message.Visited_Nodes = append(message.Visited_Nodes, id)
        logger.Info(fmt.Sprintf("Updated Visited Nodes: %s, thread id: %s\n", message.Visited_Nodes, id))
        message.Source = id
        fmt.Print("Ring struct before", ring_structure)
        go DispatchMessage(ring_structure, message, failedchan)
        return false
    }
}
func DispatchMessage(ring_structure []string, message_cont MessageWrapper, failedchan chan string) {
    // Ringstruct 2,3,4 
    // visitedNode 1,2
    messageBytes, err := json.Marshal(message_cont)
    if err != nil {
        logger.Info(fmt.Sprint("Error encoding message:", err))
        return
    }
    fmt.Print("ring struct", ring_structure)
	loop:
    for _, target := range ring_structure {
    fmt.Print("Server name for sending stuff teehee ",target, "\n")
    err:=connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: target, Message: messageBytes})
    if err != nil {
        logger.Info(fmt.Sprintf("Failed to send message to %v; retrying with next target", target))
        fmt.Printf("Error: %v\n", err)
        continue 
    }else {
        logger.Info(fmt.Sprintf("Message sent successfully to %v", target))
        break loop
    }
}
fmt.Print("Broke loop\n")
}
func HandleDiscoveryMessage(nodeIP string, ring_structure []string, message MessageWrapper, failechan chan string) {
	newmessage := Deepcopy(message)
    fmt.Print("Args being pass %s, %v, %s",ring_structure,newmessage,nodeIP)
    isComplete := Pass_message_down_ring(ring_structure, newmessage, nodeIP, failechan)
    if isComplete {
        logger.Info(fmt.Sprintf("Completed Discovery in %s: %v", nodeIP, newmessage.Visited_Nodes))
        electedCoordinator := slices.Max(newmessage.Visited_Nodes)
        logger.Info(fmt.Sprintf("New Coordinator: %v", electedCoordinator))
        ring_struct := ReorderRing(newmessage.Visited_Nodes, nodeIP)
        go SendRingAnnouncement(nodeIP, ring_struct, []string{electedCoordinator}, MessageTypeAnnouncement, failechan)
    }
}
// Processes an announcement message and initiates a new ring announcement if the election finishes.
func HandleAnnouncementMessage(nodeIP string, ring []string, message MessageWrapper, failedchan chan string) string {
    newmessage := Deepcopy(message)
    isComplete := Pass_message_down_ring(ring, newmessage, nodeIP, failedchan)
    if isComplete {
        logger.Info(fmt.Sprintf("Finish Election"))
        logger.Info(fmt.Sprintf("this all visited nodes %v",newmessage.Visited_Nodes))
        go SendRingAnnouncement(nodeIP,ring, newmessage.Visited_Nodes, MessageTypeNewRing,failedchan)
    }
    return slices.Max(newmessage.Payload)
}

func HandleNewRingMessage(ring_structure []string, message MessageWrapper, id string, failedchan chan string) []string {
	message = Deepcopy(message)
	Pass_message_down_ring( ring_structure, message, id,failedchan)
	return message.Payload
}
func HandleMessage(nodeIP string, failedChan chan string, messageWrapper MessageWrapper) {
    var coordinator string
    fmt.Print("handle message func \n", nodeIP)
    switch messageWrapper.Message_Type {
    case MessageTypeDiscovery:
        ring_structure := ReorderRing(addresses, nodeIP)
        HandleDiscoveryMessage(nodeIP, ring_structure, messageWrapper, failedChan)
    case MessageTypeAnnouncement:
        ring_structure := ReorderRing(addresses, nodeIP)
        coordinator = HandleAnnouncementMessage(nodeIP, ring_structure, messageWrapper, failedChan)
        logger.Info(fmt.Sprint(nodeIP, " acknowledges new coordinator ", coordinator))
    case MessageTypeNewRing:
        ring_structure := ReorderRing(addresses, nodeIP)
        fmt.Print("Hello new ring vistied nodes ", messageWrapper.Visited_Nodes)
        updatedRing := HandleNewRingMessage(DeepCopyRing(ring_structure), messageWrapper, nodeIP, failedChan)
        ring_structure = ReorderRing(ring_structure, nodeIP)
        fmt.Print("wjwats jeree", ring_structure)
        addresses = updatedRing
        logger.Info(fmt.Sprint("Updated ring structure for node ", nodeIP, updatedRing))
    }
}

func ElectionInit(addresses []string, address string) ([]string, chan string) {
	// Create a default ring structure based on the provided addresses
	defaultRing := make([]string, len(addresses))
	for i := 0; i < len(addresses); i++ {
		defaultRing[i] = addresses[i]
	}

	// Create a channel for failed sends
	failedSends := make(chan string)

	// Reorder the ring based on the current node's address
	ringStructure := ReorderRing(defaultRing, address)

	// Return the ring structure and the failed sends channel
	return ringStructure, failedSends
}
func Deepcopy(msg_cont MessageWrapper) MessageWrapper {
    contentCopy := make([]string, len(msg_cont.Payload))
    copy(contentCopy, msg_cont.Payload)

    visitedCopy := make([]string, len(msg_cont.Visited_Nodes))
    copy(visitedCopy, msg_cont.Visited_Nodes)

    return MessageWrapper{
		Message_Type: msg_cont.Message_Type,
        Visited_Nodes: visitedCopy,
        Source:        msg_cont.Source,
		Payload:      contentCopy,

    }
}
func DeepCopyRing(slice []string) []string {
    copied := make([]string, len(slice))
    copy(copied, slice)
    return copied
}
