package proposals

// Contains basic implementation of proposal functions. Currently just operates on a single variable as the "data".

import (
	"encoding/binary"
	"encoding/json"
	cxn "local/zookeeper/internal/LocalConnectionManager"
	"local/zookeeper/internal/znode"
	"log"
	"log/slog"
	"net"
	"reflect"
)

// TODO replace this with a system-wide variable in the actual implementation
var currentCoordinator net.Addr = &net.IPAddr{
	IP: net.ParseIP("192.168.10.1"),
}

const n_systems = 6
const debug = true

var zxidCounter ZXIDCounter
var ackCounter AckCounter = AckCounter{ackTracker: make(map[uint32]int)}
var proposalsQueue SafeQueue[Proposal]
var syncTrack SyncTracker

// Convert the epoch and count numbers to a single zxid
func getZXIDAsInt(epoch uint16, count uint16) uint32 {
	return (uint32(epoch) << 16) | uint32(count)
}

func bytesToUint32(bytes []byte) uint32 {
	return binary.NativeEndian.Uint32(bytes)
}
func uint32ToBytes(num uint32) []byte {
	bytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(bytes, num)
	return bytes
}

// Process a received proposal
func processProposal(prop Proposal, source net.Addr, failedSend chan string, selfIP net.Addr) {
	if currentCoordinator.String() != source.String() {
		// Proposal is not from the current coordinator; ignore it.
		// TODO trigger an election
		return
	}
	switch prop.PropType {
	case StateChange:
		propZxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
		slog.Info(selfIP.String(), "receives proposal with zxid", propZxid)
		zxidCounter.incCount()

		// If we're currently syncing, automatically save and commit.
		syncing, zxidCap := syncTrack.readVals()
		if syncing && propZxid <= zxidCap {
			SaveProposal(prop)
			processPropUpdate(prop.Content)
		} else {
			// Otherwise, normal operation
			proposalsQueue.enqueue(prop)
		}
	case Commit:
		processCommitProposal(selfIP)
	case NewLeader:
		// Immediately commit all held proposals
		for !proposalsQueue.isEmpty() {
			poppedProposal, _ := proposalsQueue.dequeue()
			processPropUpdate(poppedProposal.Content)
		}

		latestZxid := bytesToUint32(prop.Content)
		currentEpoch, currentCount := zxidCounter.check()
		currentZxid := getZXIDAsInt(currentEpoch, currentCount)
		// Already holding the latest proposal, just return.
		if currentZxid == latestZxid {
			return
		}

		// Need to get updates from the leader.
		syncRequest := Request{
			ReqType: Sync,
			Content: uint32ToBytes(currentZxid),
		}

		sendRequest(syncRequest, failedSend, selfIP)
	default:
		log.Fatal("Received proposal with unknown type")
	}
}

// Process a received COMMIT proposal. This doesn't actually need any arguments for now because
// the proposals are stored in a queue.
func processCommitProposal(selfIP net.Addr) {
	prop, ok := proposalsQueue.dequeue()
	if !ok {
		log.Fatal("Received COMMIT with no proposals in queue")
	}
	if debug {
		propZxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
		slog.Info(selfIP.String(), "committing proposal with zxid", propZxid)
	}
	processPropUpdate(prop.Content)
}

// Process a received ACK message in response to a proposal.
// Increments the corresponding counter in the ACK counter map, and broadcasts a commit message if
// the number of ACKs has exceeded the majority.
func processACK(prop Proposal, failedSends chan string, selfIP net.Addr) {
	zxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
	if ackCounter.incrementOrRemove(zxid, n_systems/2) {
		processCommitProposal(selfIP)
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
}

// Write a received STATE_UPDATE proposal
func processPropUpdate(writeData []byte) {
	_, err := znode.Write(writeData)
	if err != nil {
		log.Fatal("Error from znode: ", err)
	}
}

// Process a received request.
// If it is a WRITE request, propagate it to the non-coordinators as a new proposal.
// If it is a SYNC request, read the sent ZXID and send all proposals from that ZXID onwards.
func processRequest(req Request, failedSends chan string, selfIP net.Addr, remoteIP net.Addr) bool {
	switch req.ReqType {
	case Write:
		success, err := znode.Check(req.Content)
		if err != nil {
			log.Fatal("Error checking: ", err)
		}
		if !success {
			if selfIP.String() == remoteIP.String() {
				return false
			}
			reqJson, _ := json.Marshal(req)
			zabMsg := ZabMessage{
				ZabType: Err,
				Content: reqJson,
			}
			sendZabMessage(remoteIP, zabMsg, failedSends, selfIP)
			return false
		}

		epoch, count := zxidCounter.incCount()
		zxid := getZXIDAsInt(epoch, count)
		ackCounter.storeNew(zxid)
		prop := Proposal{
			StateChange,
			epoch,
			count,
			req.Content,
		}
		proposalsQueue.enqueue(prop)
		broadcastProposal(prop, selfIP, failedSends)
	case Sync:
		// TODO
	}
	return true
}

// Broadcast a request as a proposal.
func broadcastProposal(prop Proposal, selfIP net.Addr, failedSends chan string) {
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
	cxn.Broadcast(serial, selfIP, failedSends)
}

// Sends a new write request to a given machine.
func SendWriteRequest(content []byte, failedSends chan string, selfIP net.Addr) {
	req := Request{
		ReqType: Write,
		Content: content,
	}
	sendRequest(req, failedSends, selfIP)
}

// Send a Request to a machine.
// If a machine attempts to send a request to its own IP address, the request is immediately
// processed instead of being sent through the network.
func sendRequest(req Request, failedSends chan string, selfIP net.Addr) {
	if currentCoordinator == selfIP {
		processRequest(req, failedSends, selfIP, selfIP)
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
func ProcessZabMessage(netMsg cxn.NetworkMessage, failedSends chan string, selfIP net.Addr) {
	src := netMsg.Remote
	msgSerial := netMsg.Message
	var msg ZabMessage
	deserialise(msgSerial, &msg)

	// Check syncing
	if msg.ZabType == Prop {
		var prp Proposal
		deserialise(msg.Content, &prp)
		if prp.PropType == NewLeader {
			latestZxid := bytesToUint32(prp.Content)
			currentEpoch, currentCount := zxidCounter.check()
			currentZxid := getZXIDAsInt(currentEpoch, currentCount)
			// If already holding the latest proposal, just ACK
			if currentZxid < latestZxid {
				syncTrack.newSync(latestZxid, msg)
			}
		}
	}
	switch msg.ZabType {
	case Req:
		var req Request
		deserialise(msg.Content, &req)
		go processRequest(req, failedSends, selfIP, netMsg.Remote)
	case Prop:
		var prop Proposal
		deserialise(msg.Content, &prop)
		go processProposal(prop, src, failedSends, selfIP)
	case ACK:
		var ack Proposal
		deserialise(msg.Content, &ack)
		go processACK(ack, failedSends, selfIP)
	}

	// ACK section
	if msg.ZabType == Prop {
		// Only acknowledge proposals that are StateChange
		var prp Proposal
		deserialise(msg.Content, &prp)
		if prp.PropType == StateChange {
			ack := makeACK(msg)
			go sendZabMessage(src, ack, failedSends, selfIP)
		} else if prp.PropType == NewLeader {
			latestZxid := bytesToUint32(prp.Content)
			currentEpoch, currentCount := zxidCounter.check()
			currentZxid := getZXIDAsInt(currentEpoch, currentCount)
			// If already holding the latest proposal, just ACK
			if currentZxid >= latestZxid {
				ack := makeACK(msg)
				go sendZabMessage(src, ack, failedSends, selfIP)
			}
		}
	}

	// Check syncing. If synced, ack the NewLeader proposal.
	syncing, zxidCap := syncTrack.readVals()
	if syncing {
		currentEpoch, currentCount := zxidCounter.check()
		currentZxid := getZXIDAsInt(currentEpoch, currentCount)
		if zxidCap <= currentZxid {
			syncMsg := syncTrack.getStoredProposal()
			ack := makeACK(syncMsg)
			go sendZabMessage(src, ack, failedSends, selfIP)
		}
	}
}
