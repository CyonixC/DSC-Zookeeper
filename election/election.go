package election

import (
	"encoding/json"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"slices"
)
type MessageType int
var addresses = []string{"server1", "server2", "sever3"}


const (
	MessageTypeDiscovery = iota
	MessageTypeAnnouncement
	MessageTypeNewRing
	MessageTypeHeartbeat
)

type MessageWrapper struct {
	Message_Type MessageType
	Visited_Nodes []string 
	Source         string
	Payload        []string
}

func ReorderRing (ring_structure []string, id string) []string {
	startIndex := slices.Index(ring_structure, id)
	if startIndex == -1 {
		return ring_structure 
	}
	
	afterStart := ring_structure[startIndex+1:]
	beforeStart := ring_structure[:startIndex]
	
	return slices.Concat(afterStart, beforeStart)
}
// dont think need just broadcast
func SendRingAnnouncement(nodeIP string, ring []string, content []string, messageType MessageType, failedchan chan string ) {
    StartRingMessage(nodeIP, ring, content, messageType, failedchan)
}
func StartRingMessage(nodeIP string, ring_structure []string, messageContent []string, messageType MessageType, failedchan chan string) {
    visitedNodes := make([]string, 0, len(addresses))
    visitedNodes = append(visitedNodes, nodeIP)
    fmt.Printf("Start Ring Message %s\n", nodeIP)
    message := MessageWrapper{
        messageType,
        visitedNodes,
        nodeIP,
		[]string{"start ring message"},
    }
    go DispatchMessage(ring_structure,message,failedchan)
}
func DispatchMessage(ring_structure []string, message_cont MessageWrapper, failedchan chan string) {
    fmt.Printf("Dispatch Message\n")
    messageBytes, err := json.Marshal(message_cont)
    if err != nil {
        logger.Info(fmt.Sprint("Error encoding message:", err))
        return
    }
    if err != nil {
        logger.Info(fmt.Sprint("Error resolving Source address:", err))
        return
	}
	loop:
for _, target := range ring_structure {
    if err != nil {
        logger.Info(fmt.Sprint("Error resolving target address:", err))
        return
    }
    fmt.Print("Before Send")
    connectionManager.SendMessage(connectionManager.NetworkMessage{Remote: target, Message: messageBytes})
    fmt.Print("Afterr Send")

    select {
    case failedNode := <-failedchan:
        logger.Info(fmt.Sprint("Failed to send message to %s; retrying with next target\n", failedNode))

        // Continue to the next iteration, retry with the next target
    default: // Adjust the timeout duration as needed
    logger.Info(fmt.Sprint("Message sent successfully, proceeding"))
        break loop // This will break out of the for loop
    }
}
}

func InitiateElectionDiscovery(nodeIP string, ring_structure []string, failedchan chan string) {
    logger.Info(fmt.Sprintf("Node %s initiated election discovery\n", nodeIP))
    initialContent := []string{}
    StartRingMessage(nodeIP, ring_structure, initialContent, MessageTypeDiscovery, failedchan)
	
}
func Pass_message_down_ring(ring_structure []string, message MessageWrapper, id string, failedchan chan string) bool {
    if slices.Contains(message.Visited_Nodes, id) {
        logger.Info(fmt.Sprint("Node %s already visited, completing ring pass.\n", id))
        return true 
    } else {
        message.Visited_Nodes = append(message.Visited_Nodes, id)
        logger.Info(fmt.Sprint("Updated Visited Nodes: %s, thread id: %s\n", message.Visited_Nodes, id))
        message.Source = id
        go DispatchMessage(ring_structure, message, failedchan)
        return false
    }
}

func HandleDiscoveryMessage(nodeIP string, ring_structure []string, message MessageWrapper, failechan chan string) {
	newmessage := Deepcopy(message)
    isComplete := Pass_message_down_ring(ring_structure, newmessage, nodeIP, failechan)
    if isComplete {
        logger.Info(fmt.Sprint("Completed Discovery in", nodeIP, ":", newmessage.Visited_Nodes))
        electedCoordinator := slices.Max(newmessage.Visited_Nodes)
        logger.Info(fmt.Sprint("New Coordinator", electedCoordinator))
        go SendRingAnnouncement(nodeIP, ring_structure, []string{electedCoordinator}, MessageTypeAnnouncement, failechan)
    }
}
// Processes an announcement message and initiates a new ring announcement if the election finishes.
func HandleAnnouncementMessage(nodeIP string, ring []string, message MessageWrapper, failedchan chan string) string {
    newmessage := Deepcopy(message)
    isComplete := Pass_message_down_ring(ring, newmessage, nodeIP, failedchan)
    if isComplete {
        logger.Info(fmt.Sprint("Finish Election"))
        go SendRingAnnouncement(nodeIP,ring, newmessage.Visited_Nodes, MessageTypeNewRing,failedchan)
    }
    return newmessage.Payload[0]
}

func HandleNewRingMessage(ring_structure []string, message MessageWrapper, id string, failedchan chan string) []string {
	message = Deepcopy(message)
	// Pass the message down the ring_structure
	Pass_message_down_ring( ring_structure, message, id,failedchan)
	
	return message.Payload
}
func HandleMessage(nodeIP string, ring_structure []string, failedChan chan string, messageWrapper MessageWrapper) {
    var coordinator string
    // Process the message based on its type
    switch messageWrapper.Message_Type {
    case MessageTypeDiscovery:
        HandleDiscoveryMessage(nodeIP, ring_structure, messageWrapper, failedChan)
    case MessageTypeAnnouncement:
        coordinator = HandleAnnouncementMessage(nodeIP, ring_structure, messageWrapper, failedChan)
        logger.Info(fmt.Sprint(nodeIP, "acknowledges new coordinator", coordinator))
    case MessageTypeNewRing:
        updatedRing := HandleNewRingMessage(ring_structure, messageWrapper, nodeIP, failedChan)
        ring_structure = ReorderRing(updatedRing, nodeIP)
        logger.Info(fmt.Sprint("Updated ring structure for node", nodeIP, ring_structure))
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
