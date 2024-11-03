package proposals

// Contains basic implementation of proposal functions. Currently just operates on a single variable as the "data".

import (
	"encoding/json"
	"fmt"
	cxn "local/zookeeper/internal/LocalConnectionManager"
	"log"
	"net"
	"reflect"
	"sync"
)

// TODO replace this with a system-wide variable in the actual implementation
var currentCoordinator net.Addr = &net.IPAddr{
	IP: net.ParseIP("192.168.10.1"),
}

const n_systems = 5

var zxidCounter ZXIDCounter
var ackCounter AckCounter = AckCounter{ackTracker: make(map[uint32]int)}
var proposalsQueue SafeQueue[Proposal]
var lockedVar = struct {
	sync.RWMutex
	num int
}{
	sync.RWMutex{},
	0,
}

// Convert the epoch and count numbers to a single zxid
func getZXIDAsInt(epoch uint16, count uint16) uint32 {
	return (uint32(epoch) << 16) | uint32(count)
}

// Process a received proposal
func processProposal(prop Proposal, source net.Addr) {
	if currentCoordinator.String() != source.String() {
		// Proposal is not from the current coordinator; ignore it.
		return
	}
	switch prop.PropType {
	case StateChange:
		proposalsQueue.enqueue(prop)
	case Commit:
		processCommitProposal()
	case NewLeader:
		// TODO
		return
	default:
		log.Fatal("Received proposal with unknown type")
	}
}

// Process a received COMMIT proposal. This doesn't actually need any arguments for now because
// the proposals are stored in a queue.
func processCommitProposal() {
	prop, ok := proposalsQueue.dequeue()
	if !ok {
		log.Fatal("Received COMMIT with no proposals in queue")
	}
	processPropUpdate(int(prop.Content[0]))
}

// Process a received ACK message in response to a proposal.
// Increments the corresponding counter in the ACK counter map, and broadcasts a commit message if
// the number of ACKs has exceeded the majority.
func processACK(prop Proposal, failedSends chan string, selfIP net.Addr) {
	zxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
	log.Println(selfIP, "receives ACK for", zxid)
	if ackCounter.incrementOrRemove(zxid, n_systems/2) {
		broadcastCommit(failedSends, selfIP)
	}
}

// Broadcast a new COMMIT message to all non-coordinators.
func broadcastCommit(failedSends chan string, selfIP net.Addr) {
	// leave everything empty except commit for now (assume TCP helps us with ordering)
	prop := Proposal{
		Commit,
		0,
		0,
		nil,
	}
	propJson, err := json.Marshal(prop)
	if err != nil {
		log.Fatal("Error on JSON conversion", err)
	}
	zab := ZabMessage{
		Prop,
		propJson,
	}
	broadcastZabMessage(zab, selfIP, failedSends)
	// log.Println("Broadcasting commit message: ", zab)
}

// Process a received STATE_UPDATE proposal. Just store it in the queue.
func processPropUpdate(update int) {
	lockedVar.Lock()
	lockedVar.num = update
	fmt.Println(lockedVar.num)
	lockedVar.Unlock()
}

// Process a received request.
// If it is a WRITE request, propagate it to the non-coordinators as a new proposal.
// If it is a SYNC request, read the sent ZXID and send all proposals from that ZXID onwards.
func processRequest(req Request, failedSends chan string, selfIP net.Addr) {
	// log.Println(selfIP, "received request")
	switch req.ReqType {
	case Write:
		epoch, count := zxidCounter.incCount()
		zxid := getZXIDAsInt(epoch, count)
		ackCounter.storeNew(zxid)
		log.Println(selfIP, "broadcasting request")
		broadcastProposal(req, epoch, count, selfIP, failedSends)
	case Sync:
		// TODO
	}
}

// Broadcast a request as a proposal.
func broadcastProposal(req Request, epoch uint16, count uint16, selfIP net.Addr, failedSends chan string) {
	prop := Proposal{
		StateChange,
		epoch,
		count,
		req.Content,
	}
	propJson, err := json.Marshal(prop)
	if err != nil {
		log.Fatal("Error on JSON conversion", err)
	}
	zab := ZabMessage{
		Prop,
		propJson,
	}
	broadcastZabMessage(zab, selfIP, failedSends)
}

// Construct an ACK message from a Zab message.
// Basically just the exact same package, but the type is ACK instead.
// Used for ACKing proposals; this is NOT a general purpose ACK!
func makeACK(msg ZabMessage) (ack ZabMessage) {
	ack.Content = make([]byte, len(msg.Content))
	copy(ack.Content, msg.Content)
	ack.ZabType = ACK
	return
}

// Un-JSON-ify a JSON data slice into a Zab message type.
func deserialise[m Deserialisable](serialised []byte, msgPtr *m) {
	err := json.Unmarshal(serialised, msgPtr)
	if err != nil {
		log.Fatal("Could not convert ", reflect.TypeOf(msgPtr), " from bytes: ", err)
	}
}

// Send a Zab message over the network.
func sendZabMessage(dest net.Addr, msg ZabMessage, failedSend chan string, selfIP net.Addr) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	cxn.SendMessage(cxn.NetworkMessage{Remote: dest, Message: serial}, selfIP, failedSend)
}

// Broadcast a Zab message over the network.
func broadcastZabMessage(msg ZabMessage, selfIP net.Addr, failedSends chan string) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
		return
	}
	// log.Println(selfIP, "broadcasting network msg")
	cxn.Broadcast(serial, selfIP, failedSends)
}

// Send a Request to a machine.
// If a machine attempts to send a request to its own IP address, the request is immediately
// processed instead of being sent through the network.
func SendRequest(req Request, failedSends chan string, selfIP net.Addr) {
	if currentCoordinator == selfIP {
		processRequest(req, failedSends, selfIP)
		return
	}

	serial, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
		return
	}
	msg := ZabMessage{
		Req,
		serial,
	}
	sendZabMessage(currentCoordinator, msg, failedSends, selfIP)
}

// Process a Zab message received from the network.
func ProcessZabMessage(src net.Addr, msgSerial []byte, failedSends chan string, selfIP net.Addr) {
	var msg ZabMessage
	deserialise(msgSerial, &msg)
	if msg.ZabType == Prop {
		// Only acknowledge proposals that are not COMMIT proposals
		var prp Proposal
		deserialise(msg.Content, &prp)
		if prp.PropType != Commit {
			ack := makeACK(msg)
			go sendZabMessage(src, ack, failedSends, selfIP)
		}
	}
	switch msg.ZabType {
	case Req:
		var req Request
		deserialise(msg.Content, &req)
		go processRequest(req, failedSends, selfIP)
	case Prop:
		var prop Proposal
		deserialise(msg.Content, &prop)
		go processProposal(prop, src)
	case ACK:
		var ack Proposal
		deserialise(msg.Content, &ack)
		go processACK(ack, failedSends, selfIP)
	}
}
