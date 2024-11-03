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

const (
	Req ZabMessageType = iota
	Prop
	ACK
)

const (
	Commit ProposalType = iota
	StateChange
	NewLeader
)

const (
	Sync RequestType = iota
	Write
)

type Deserialisable interface {
	ZabMessage | Proposal | Request
}

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

type ProposalType int
type ZabMessageType int
type RequestType int
type Proposal struct {
	PropType ProposalType
	EpochNum uint16
	CountNum uint16
	Content  []byte
}
type Request struct {
	ReqType RequestType
	Content []byte
}
type ZabMessage struct {
	ZabType ZabMessageType
	Content []byte
}
type ZXIDCounter struct {
	sync.RWMutex
	epochNum uint16
	countNum uint16
}
type AckCounter struct {
	sync.Mutex
	ackTracker map[uint32]int
}

func (cnt *ZXIDCounter) check() (epochNum uint16, countNum uint16) {
	cnt.RLock()
	defer cnt.RUnlock()
	epochNum = cnt.epochNum
	countNum = cnt.countNum
	return
}
func (cnt *ZXIDCounter) incCount() (epochNum uint16, countNum uint16) {
	cnt.Lock()
	defer cnt.Unlock()
	cnt.countNum++
	epochNum = cnt.epochNum
	countNum = cnt.countNum
	return
}
func (cnt *ZXIDCounter) incEpoch() (epochNum uint16, countNum uint16) {
	cnt.Lock()
	defer cnt.Unlock()
	cnt.countNum = 0
	cnt.epochNum++
	epochNum = cnt.epochNum
	countNum = cnt.countNum
	return
}

func getZXIDAsInt(epoch uint16, count uint16) uint32 {
	return (uint32(epoch) << 16) | uint32(count)
}

func (ackCnt *AckCounter) increment(zxid uint32) int {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	count, ok := ackCnt.ackTracker[zxid]
	if !ok {
		return 0
	}
	ackCnt.ackTracker[zxid] = count + 1
	return count + 1
}

func (ackCnt *AckCounter) remove(zxid uint32) {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	delete(ackCnt.ackTracker, zxid)
}

func (ackCnt *AckCounter) incrementOrRemove(zxid uint32, maxCount int) bool {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	count, ok := ackCnt.ackTracker[zxid]
	if !ok {
		return false
	}
	if count+1 > maxCount {
		ackCnt.ackTracker[zxid] = 0
		return true
	}

	ackCnt.ackTracker[zxid] = count + 1
	return false
}

func (ackCnt *AckCounter) storeNew(zxid uint32) {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	ackCnt.ackTracker[zxid] = 1
}

func receiveProposal(prop Proposal, source net.Addr) {
	if currentCoordinator.String() != source.String() {
		// Proposal is not from the current coordinator; ignore it.
		return
	}
	// log.Println("Received proposal from", source, "of type", prop.PropType)
	switch prop.PropType {
	case StateChange:
		proposalsQueue.enqueue(prop)
	case Commit:
		receiveCommitProp()
	case NewLeader:
		return
	default:
		log.Fatal("Received proposal with unknown type")
	}
}

func receiveCommitProp() {
	prop, ok := proposalsQueue.dequeue()
	// log.Println("Received commit, popping proposal", prop)
	if !ok {
		log.Fatal("Received COMMIT with no proposals in queue")
	}
	processPropUpdate(int(prop.Content[0]))
}

func receiveACK(prop Proposal, failedSends chan string, selfIP net.Addr) {
	zxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
	log.Println(selfIP, "receives ACK for", zxid)
	if ackCounter.incrementOrRemove(zxid, n_systems/2) {
		broadcastCommit(failedSends, selfIP)
	}
}

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

func processPropUpdate(update int) {
	lockedVar.Lock()
	lockedVar.num = update
	fmt.Println(lockedVar.num)
	lockedVar.Unlock()
}

func receiveRequest(req Request, failedSends chan string, selfIP net.Addr) {
	// log.Println(selfIP, "received request")
	switch req.ReqType {
	case Write:
		epoch, count := zxidCounter.incCount()
		zxid := getZXIDAsInt(epoch, count)
		ackCounter.storeNew(zxid)
		log.Println(selfIP, "broadcasting request")
		broadcastRequest(req, epoch, count, selfIP, failedSends)
	case Sync:
		// TODO
	}
}

func broadcastRequest(req Request, epoch uint16, count uint16, selfIP net.Addr, failedSends chan string) {
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
	// log.Println(selfIP, "broadcasting zab", zab)
	// log.Println(selfIP, "broadcasting prop", prop)
	broadcastZabMessage(zab, selfIP, failedSends)
}

func makeACK(msg ZabMessage) (ack ZabMessage) {
	ack.Content = make([]byte, len(msg.Content))
	copy(ack.Content, msg.Content)
	ack.ZabType = ACK
	return
}

func deserialise[m Deserialisable](serialised []byte, msgPtr *m) {
	err := json.Unmarshal(serialised, msgPtr)
	if err != nil {
		log.Fatal("Could not convert ", reflect.TypeOf(msgPtr), " from bytes: ", err)
	}
}

func sendZabMessage(dest net.Addr, msg ZabMessage, failedSend chan string, selfIP net.Addr) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	cxn.SendMessage(cxn.NetworkMessage{Remote: dest, Message: serial}, selfIP, failedSend)
}
func broadcastZabMessage(msg ZabMessage, selfIP net.Addr, failedSends chan string) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
		return
	}
	// log.Println(selfIP, "broadcasting network msg")
	cxn.Broadcast(serial, selfIP, failedSends)
}

func SendRequest(req Request, failedSends chan string, selfiP net.Addr) {
	if currentCoordinator == selfiP {
		receiveRequest(req, failedSends, selfiP)
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
	sendZabMessage(currentCoordinator, msg, failedSends, selfiP)
}

func ReceiveZabMessage(src net.Addr, msgSerial []byte, failedSends chan string, selfIP net.Addr) {
	var msg ZabMessage
	deserialise(msgSerial, &msg)
	// log.Println(selfIP, "received ZAB message from", src, ": ", msg.Content)
	if msg.ZabType == Prop {
		// Acknowledge any non-commit proposals
		var prp Proposal
		deserialise(msg.Content, &prp)
		if prp.PropType != Commit {
			ack := makeACK(msg)
			go sendZabMessage(src, ack, failedSends, selfIP)
		}
	} else {
		// log.Println(selfIP, "received message from", src, "of type", msg.ZabType)
	}
	switch msg.ZabType {
	case Req:
		var req Request
		deserialise(msg.Content, &req)
		go receiveRequest(req, failedSends, selfIP)
	case Prop:
		var prop Proposal
		deserialise(msg.Content, &prop)
		go receiveProposal(prop, src)
	case ACK:
		var ack Proposal
		deserialise(msg.Content, &ack)
		go receiveACK(ack, failedSends, selfIP)
	}
}
