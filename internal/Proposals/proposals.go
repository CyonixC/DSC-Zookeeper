package proposals

// Contains basic implementation of proposal functions. Currently just operates on a single variable as the "data".

import (
	"encoding/json"
	"fmt"
	"log"
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

// TODO replace this with a system-wide variable in the actual implementation
const currentCoordinator = 0

var zxidCounter ZXIDCounter
var ackCounter AckCounter
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

func (ackCnt *AckCounter) storeNew(zxid uint32) {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	ackCnt.ackTracker[zxid] = 1
}

func receiveProposal(prop Proposal, source int) {
	if currentCoordinator != source {
		// Proposal is not from the current coordinator; ignore it.
		return
	}
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
	if !ok {
		log.Fatal("Received COMMIT with no proposals in queue")
	}
	processPropUpdate(int(prop.Content[0]))
}

func processPropUpdate(update int) {
	lockedVar.Lock()
	lockedVar.num = update
	fmt.Println(lockedVar.num)
	lockedVar.Unlock()
}

func receiveRequest(req Request) {
	switch req.ReqType {
	case Write:
		epoch, count := zxidCounter.incCount()
		zxid := getZXIDAsInt(epoch, count)
		ackCounter.storeNew(zxid)
		broadcastRequest(req, epoch, count)
	case Sync:
		// TODO
	}
}

func broadcastRequest(req Request, epoch uint16, count uint16) {
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
	broadcastZabMessage(zab)
}

func makeACK(msg ZabMessage) (ack ZabMessage) {
	copy(ack.Content, msg.Content)
	ack.ZabType = ACK
	return
}

func sendZabMessage(dest int, msg ZabMessage)
func broadcastZabMessage(msg ZabMessage)
func receiveZabMessage(src int, msg ZabMessage) {
	ack := makeACK(msg)
	propJSON, err := json.Marshal(ack)
	if err != nil {
		log.Fatal("Error on JSON conversion", err)
	}
	message := ZabMessage{Prop, propJSON}
	go sendZabMessage(0, message)
}
