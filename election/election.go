package main

import (
	"encoding/json"
	"fmt"
	localconnectionmanager "local/zookeeper/internal/LocalConnectionManager"
	"net"
	"slices"
)
type MessageType int
var addresses = []string{"192.168.10.1", "192.168.10.2", "192.168.10.3"}


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

func reorderRing (ring_structure []string, id string) []string {
	startIndex := slices.Index(ring_structure, id)
	if startIndex == -1 {
		return ring_structure 
	}
	
	afterStart := ring_structure[startIndex+1:]
	beforeStart := ring_structure[:startIndex]
	
	return slices.Concat(afterStart, beforeStart)
}
// dont think need just broadcast
func sendRingAnnouncement(nodeIP string, ring []string, content []string, messageType MessageType, failedchan chan string ) {
    startRingMessage(nodeIP, ring, content, messageType, failedchan)
}
func startRingMessage(nodeIP string, ring_structure []string, messageContent []string, messageType MessageType, failedchan chan string) {
    visitedNodes := make([]string, 0, len(addresses))
    visitedNodes = append(visitedNodes, nodeIP)
    message := MessageWrapper{
        messageType,
        visitedNodes,
        nodeIP,
		[]string{"start ring message"},
    }
    go dispatchMessage(ring_structure,message,failedchan)
}
func dispatchMessage(ring_structure []string, message_cont MessageWrapper, failedchan chan string) {
    messageBytes, err := json.Marshal(message_cont)
    if err != nil {
        fmt.Println("Error encoding message:", err)
        return
    }
    tcpAddr, err := net.ResolveIPAddr("ip", message_cont.Source)
    if err != nil {
        fmt.Println("Error resolving Source address:", err)
        return
	}
	loop:
for _, target := range ring_structure {
    ipAddr, err := net.ResolveIPAddr("ip", target)
    if err != nil {
        fmt.Println("Error resolving target address:", err)
        return
    }

    localconnectionmanager.SendMessage(localconnectionmanager.NetworkMessage{Remote: ipAddr, Message: messageBytes}, tcpAddr, failedchan)

    // Wait for a potential failure or timeout
    select {
    case failedNode := <-failedchan:
        fmt.Printf("Failed to send message to %s; retrying with next target\n", failedNode)
        // Continue to the next iteration, retry with the next target
    default: // Adjust the timeout duration as needed
        fmt.Println("Message sent successfully, proceeding")
        break loop // This will break out of the for loop
    }
}
}

func initiateElectionDiscovery(nodeIP string, ring_structure []string, failedchan chan string) {
    fmt.Printf("Node %d initiated election discovery\n", nodeIP)
    initialContent := []string{}
    startRingMessage(nodeIP, ring_structure, initialContent, MessageTypeDiscovery, failedchan)
	
}
func pass_message_down_ring(ring_structure []string, message MessageWrapper, id string, failedchan chan string) bool {
    if slices.Contains(message.Visited_Nodes, id) {
        fmt.Printf("Node %s already visited, completing ring pass.\n", id)
        return true 
    } else {
        message.Visited_Nodes = append(message.Visited_Nodes, id)
        fmt.Printf("Updated Visited Nodes: %s, thread id: %s\n", message.Visited_Nodes, id)
        message.Source = id
        go dispatchMessage(ring_structure, message, failedchan)
        return false
    }
}

func handleDiscoveryMessage(nodeIP string, ring_structure []string, message MessageWrapper, failechan chan string) {
	newmessage := deepcopy(message)
    isComplete := pass_message_down_ring(ring_structure, newmessage, nodeIP, failechan)
    if isComplete {
        fmt.Println("Completed Discovery in", nodeIP, ":", newmessage.Visited_Nodes)
        electedCoordinator := slices.Max(newmessage.Visited_Nodes)
        fmt.Println("New Coordinator", electedCoordinator)
        go sendRingAnnouncement(nodeIP, ring_structure, []string{electedCoordinator}, MessageTypeAnnouncement, failechan)
    }
}
// Processes an announcement message and initiates a new ring announcement if the election finishes.
func handleAnnouncementMessage(nodeIP string, ring []string, message MessageWrapper, failedchan chan string) string {
    newmessage := deepcopy(message)
    isComplete := pass_message_down_ring(ring, newmessage, nodeIP, failedchan)
    if isComplete {
        fmt.Println("Finish Election")
        go sendRingAnnouncement(nodeIP,ring, newmessage.Visited_Nodes, MessageTypeNewRing,failedchan)
    }
    return newmessage.Payload[0]
}

func handleNewRingMessage(ring_structure []string, message MessageWrapper, id string, failedchan chan string) []string {
	message = deepcopy(message)
	// Pass the message down the ring_structure
	pass_message_down_ring( ring_structure, message, id,failedchan)
	
	return message.Payload
}
func client(nodeIP string, ring_structure []string, failedChan chan string) {
	var coordinator string = ""
	netaddr, _ := net.ResolveIPAddr("ip", nodeIP)
	recv_channel := localconnectionmanager.Init(netaddr)
	print(nodeIP)
	if nodeIP=="192.168.10.3"{
		fmt.Printf("Starting election\n")
		initiateElectionDiscovery(nodeIP, ring_structure, failedChan)
	}

	for {
		select {
		case receivedMsg := <-recv_channel:
			var messageWrapper MessageWrapper
            err := json.Unmarshal(receivedMsg.Message, &messageWrapper)
            if err != nil {
                fmt.Println("Error unmarshaling message:", err)
                continue
            }
            switch messageWrapper.Message_Type {
			case MessageTypeDiscovery:
				handleDiscoveryMessage(nodeIP,ring_structure, messageWrapper, failedChan)
			case MessageTypeAnnouncement:
				coordinator = handleAnnouncementMessage(nodeIP,ring_structure, messageWrapper, failedChan)
				fmt.Println(nodeIP, "acknowledges new coordinator", coordinator)			
			case MessageTypeNewRing:
				updatedRing := handleNewRingMessage(ring_structure, messageWrapper, nodeIP, failedChan)
				ring_structure = reorderRing (updatedRing, nodeIP)
				fmt.Println("Updated ring structure for node", nodeIP, ring_structure)
			}

		}
		
	}
}
func deepcopy(msg_cont MessageWrapper) MessageWrapper {
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

func main() {
	defaultRing  := make([]string, len(addresses))
	for i := 0; i < len(addresses); i++ {
		defaultRing [i] = addresses[i]
	}

	for i := 0; i < len(addresses); i++ {
		failedSends := make(chan string)
		ring_structure := reorderRing (defaultRing ,addresses[i])
		go client(string(addresses[i]), ring_structure, failedSends)
	}
	select{}

}