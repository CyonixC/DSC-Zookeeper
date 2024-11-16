package proposals

// Contains basic implementation of proposal functions. Currently just operates on a single variable as the "data".

import (
	"encoding/json"
	"fmt"
	cxn "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log"
	"os"
)

var currentCoordinator string = "server1"

const n_systems = 3

var zxidCounter ZXIDCounter
var ackCounter = AckCounter{ackTracker: make(map[uint32]int)}
var proposalsQueue SafeQueue[Proposal]
var syncTrack SyncTracker

var newProposalChan = make(chan Proposal, 10)
var toCommitChan = make(chan []byte, 10)
var toSendChan = make(chan ToSendMessage, 10)
var failedRequestChan = make(chan Request)

type checkFunction func([]byte) ([]byte, error)

var requestChecker checkFunction

func Init(check checkFunction) (committed chan []byte, denied chan Request) {
	go proposalWriter(newProposalChan)
	go messageSender(toSendChan)
	committed = toCommitChan
	requestChecker = check
	denied = failedRequestChan
	return
}

// Process a Zab message received from the network.
func ProcessZabMessage(netMsg cxn.NetworkMessage) {
	src := netMsg.Remote
	msgSerial := netMsg.Message
	var msg ZabMessage
	deserialise(msgSerial, &msg)

	switch msg.ZabType {
	case Req:
		var req Request
		deserialise(msg.Content, &req)
		processRequest(req, netMsg.Remote)
	case Prop:
		var prop Proposal
		deserialise(msg.Content, &prop)
		processProposal(prop, src, msg)
	case ACK:
		var ack Proposal
		deserialise(msg.Content, &ack)
		processACK(ack)
	case Err:
		logger.Debug("Received ERROR")
		var originalReq Request
		deserialise(msg.Content, &originalReq)
		failedRequestChan <- originalReq

	case SyncErr:
		logger.Error(fmt.Sprint("Received SYNC ERROR! Check if current coordinator is correct: ", currentCoordinator))
	}

	// ACK section
	if msg.ZabType == Prop {
		// Only acknowledge proposals that are StateChange
		var prp Proposal
		deserialise(msg.Content, &prp)
	}

}

// Sends a new write request to a given machine.
func SendWriteRequest(content []byte, requestNum int) {
	req := Request{
		ReqType:   Write,
		ReqNumber: requestNum,
		Content:   content,
	}
	sendRequest(req)
}

// Process a received proposal
func processProposal(prop Proposal, source string, originalMsg ZabMessage) {
	if currentCoordinator != source {
		// Proposal is not from the current coordinator; ignore it.
		// TODO trigger an election
		return
	}
	switch prop.PropType {
	case StateChange:
		processStateChangeProposal(prop, source, originalMsg)
	case Commit:
		processCommitProposal()
	case NewLeader:
		processNewLeaderProposal(prop, source, originalMsg)
	default:
		logger.Fatal("Received proposal with unknown type")
	}
}

// Process a received StateChange proposal
func processStateChangeProposal(prop Proposal, source string, originalMsg ZabMessage) {
	propZxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
	logger.Info(fmt.Sprint("receives proposal with zxid", propZxid))
	epoch := prop.EpochNum
	count := prop.CountNum
	zxidCounter.setVals(epoch, count)

	// If we're currently syncing, automatically save and commit, and don't ACK.
	syncing, zxidCap := syncTrack.readVals()
	if syncing {
		queueWriteProposal(prop)
		queueCommitProposal(prop)
		// If synced, ack the NewLeader proposal.
		currentZxid := getZXIDAsInt(epoch, count)
		if zxidCap <= currentZxid {
			syncMsg := syncTrack.getStoredProposal()
			ack := makeACK(syncMsg)
			queueSend(ack, false, source)
		}
	} else {
		// Otherwise, normal operation
		ack := makeACK(originalMsg)
		queueSend(ack, false, source)
		proposalsQueue.enqueue(prop)
	}
}

// Process a received COMMIT proposal. This doesn't actually need any arguments for now because
// the proposals are stored in a queue.
func processCommitProposal() {
	prop, ok := proposalsQueue.dequeue()
	if !ok {
		logger.Fatal("Received COMMIT with no proposals in queue")
	}
	propZxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
	logger.Debug(fmt.Sprint("Committing proposal with zxid ", propZxid))
	queueCommitProposal(prop)
}

func processNewLeaderProposal(prop Proposal, source string, originalMsg ZabMessage) {
	// Immediately commit all held proposals
	for !proposalsQueue.isEmpty() {
		poppedProposal, _ := proposalsQueue.dequeue()
		queueCommitProposal(poppedProposal)
	}

	latestZxid := bytesToUint32(prop.Content)
	currentEpoch, currentCount := zxidCounter.check()
	currentZxid := getZXIDAsInt(currentEpoch, currentCount)
	// Already holding the latest proposal, just ACK and return.
	if currentZxid == latestZxid {
		ack := makeACK(originalMsg)
		queueSend(ack, false, source)
	}

	// Otherwise, need to get updates from the leader.
	// Turn on Sync (autocommit) mode
	syncTrack.newSync(latestZxid, originalMsg)
	syncRequest := Request{
		ReqType: Sync,
		Content: uint32ToBytes(currentZxid),
	}

	sendRequest(syncRequest)
}

// Process a received ACK message in response to a proposal.
// Increments the corresponding counter in the ACK counter map, and broadcasts a commit message if
// the number of ACKs has exceeded the majority.
func processACK(prop Proposal) {
	zxid := getZXIDAsInt(prop.EpochNum, prop.CountNum)
	if ackCounter.incrementOrRemove(zxid, n_systems/2) {
		processCommitProposal()
		broadcastCommit()
	}
}

// Broadcast a new COMMIT message to all non-coordinators.
func broadcastCommit() {
	// leave everything empty except commit for now (assume TCP helps us with ordering)
	prop := Proposal{
		Commit,
		0,
		0,
		nil,
	}
	propJson, err := json.Marshal(prop)
	if err != nil {
		log.Fatal(fmt.Sprint("Error on JSON conversion", err))
	}
	zab := ZabMessage{
		Prop,
		propJson,
	}
	queueSend(zab, true, "")
}

// Process a received request.
// If it is a WRITE request, propagate it to the non-coordinators as a new proposal.
// If it is a SYNC request, read the sent ZXID and send all proposals from that ZXID onwards.
// If the request check with the znode cache fails, send an error message or just return false if on the same machine.
func processRequest(req Request, remoteID string) error {
	name := os.Getenv("NAME")
	switch req.ReqType {
	case Write:
		updatedReq, err := requestChecker(req.Content)
		if err != nil {
			logger.Error(fmt.Sprint("Error processing request ", err))
			if name == remoteID {
				return err
			}
			errReq := Request{
				ReqType:   req.ReqType,
				ReqNumber: req.ReqNumber,
				Content:   updatedReq,
			}
			errJson, err := json.Marshal(errReq)
			if err != nil {
				logger.Fatal(fmt.Sprint("Error marshaling denied request ", err))
			}
			zabMsg := ZabMessage{
				ZabType: Err,
				Content: errJson,
			}
			queueSend(zabMsg, false, remoteID)
			return err
		}

		epoch, count := zxidCounter.incCount()
		zxid := getZXIDAsInt(epoch, count)
		ackCounter.storeNew(zxid)
		prop := Proposal{
			StateChange,
			epoch,
			count,
			updatedReq,
		}
		proposalsQueue.enqueue(prop)
		logger.Debug(fmt.Sprint("Request processed successfully, broadcasting as Proposal #", zxid))
		broadcastProposal(prop)
	case Sync:
		// Don't check coodinator status, just send to make it simple
		highestEpoch, highestCount := zxidCounter.check()
		sentzxid := bytesToUint32(req.Content)
		sentEpoch, count := decomposeZXID(sentzxid)

		// Sent epoch shouldn't be more than the current one.
		if sentEpoch > highestEpoch {
			replySyncRequestError(req, remoteID)
		}

		// Need to send all proposals from the sync request number onwards.
		// If the epoch number is less, need to send epoch by epoch.
		go func() {
			for epoch := sentEpoch; epoch < highestEpoch; epoch++ {
				numProposals, err := getEpochHighestCount(epoch)
				if err != nil {
					logger.Error(fmt.Sprint("Failed to read proposals from proposal store ", err))
					replySyncRequestError(req, remoteID)
				}

				numLackingProposals := int(numProposals) - int(count)

				proposalsToSend, err := ReadProposals(epoch, int(numLackingProposals))
				if err != nil {
					logger.Error(fmt.Sprint("Failed to read proposals from proposal store ", err))
					replySyncRequestError(req, remoteID)
				}

				for _, sendingProposal := range proposalsToSend {
					proposalJson, err := json.Marshal(sendingProposal)
					if err != nil {
						logger.Error(fmt.Sprint("Failed to marshal proposal from store ", err))
						replySyncRequestError(req, remoteID)
					}
					zabMsg := ZabMessage{
						ZabType: Prop,
						Content: proposalJson,
					}
					queueSend(zabMsg, false, remoteID)
				}
				count = 0
			}

			// Now at the current epoch.
			numLackingProposals := int(highestCount) - int(count)

			proposalsToSend, err := ReadProposals(highestEpoch, int(numLackingProposals))
			if err != nil {
				logger.Error(fmt.Sprint("Failed to read proposals from proposal store ", err))
				replySyncRequestError(req, remoteID)
			}

			for _, sendingProposal := range proposalsToSend {
				proposalJson, err := json.Marshal(sendingProposal)
				if err != nil {
					logger.Error(fmt.Sprint("Failed to marshal proposal from store ", err))
					replySyncRequestError(req, remoteID)
				}
				zabMsg := ZabMessage{
					ZabType: Prop,
					Content: proposalJson,
				}
				queueSend(zabMsg, false, remoteID)
			}
		}()
	}
	return nil
}

func replySyncRequestError(req Request, remoteID string) {
	reqErr, err := json.Marshal(req)
	if err != nil {
		logger.Error(fmt.Sprint("Failed to marshal failed request ", err))
	}
	zabMsg := ZabMessage{
		ZabType: SyncErr,
		Content: reqErr,
	}
	queueSend(zabMsg, false, remoteID)

}

// Broadcast a request as a proposal.
func broadcastProposal(prop Proposal) {
	propJson, err := json.Marshal(prop)
	if err != nil {
		log.Fatal(fmt.Sprint("Error on JSON conversion", err))
	}
	zab := ZabMessage{
		Prop,
		propJson,
	}
	queueSend(zab, true, "")
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

// Send a Request to a machine.
// If a machine attempts to send a request to its own IP address, the request is immediately
// processed instead of being sent through the network.
func sendRequest(req Request) {
	name := os.Getenv("NAME")
	if currentCoordinator == name {
		err := processRequest(req, name)
		if err != nil {
			logger.Error(fmt.Sprint("Error processing request - ", err))
		}
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
	queueSend(msg, false, currentCoordinator)
}

func queueSend(zabMsg ZabMessage, isBroadcast bool, remote string) {
	toSendMsg := ToSendMessage{
		msg:       zabMsg,
		broadcast: isBroadcast,
		target:    remote,
	}
	toSendChan <- toSendMsg
}

func queueWriteProposal(prop Proposal) {
	newProposalChan <- prop
}

func queueCommitProposal(prop Proposal) {
	toCommitChan <- prop.Content
}

// Send a Zab message over the network.
func sendZabMessage(dest string, msg ZabMessage) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	cxn.SendMessage(cxn.NetworkMessage{Remote: dest, Message: serial})
}

// Broadcast a Zab message over the network.
func broadcastZabMessage(msg ZabMessage) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
		return
	}
	cxn.Broadcast(serial)
}
