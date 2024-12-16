package proposals

// Contains basic implementation of proposal functions. Currently just operates on a single variable as the "data".

import (
	"encoding/json"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	cxn "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/election"
	"local/zookeeper/internal/logger"
	"log"
)

var n_systems int

var zxidCounter ZXIDCounter                                   // tracker for current zxid label
var ackCounter = AckCounter{ackTracker: make(map[uint32]int)} // map from zxid to number of acks received
var proposalsQueue SafeQueue[Proposal]                        // queue of proposals which have yet to be acked
var syncTrack SyncTracker                                     // tracker for non-leaders to track their sync status
var syncResponseTracker SafeCounter                           // counter for leader to count number of sync responses
var messageQueue SafeQueue[cxn.NetworkMessage]                // queue to hold incoming messages to be processed
var requestQueue SafeQueue[ZabMessage]                        // queue to hold outgoing requests to be sent
var holdingRequestsQueue SafeQueue[cxn.NetworkMessage]        // queue to hold incoming new requests to be processed, while sync process is ongoing
var sendingQueue SafeQueue[ToSendMessage]                     // queue to hold the outgoing messages to be sent
var currentSync Proposal                                      // holder for leader to track currently active sync
var syncID int                                                // jank ID system to prevent syncTimeout from timing out a different sync session

var newProposalChan = make(chan Proposal, 10)
var toCommitChan = make(chan Request, 10)
var failedRequestChan = make(chan Request)
var SyncFinish = make(chan bool, 10)

type checkFunction func([]byte) ([]byte, error)

var requestChecker checkFunction
var requestsEnabled bool
var requestsProcessingEnabled bool
var syncing bool = false

// Initialise the background goroutines for handling proposals. This Init function takes:
// - A channel which NetworkMessages arrive on from the network
// - A "check" function which takes in data in a proposal and returns modified data to be used. This function returns an error if
// the check fails.
func Init(check checkFunction) (committed chan Request, denied chan Request, counter *ZXIDCounter) {
	go proposalWriter(newProposalChan)
	go messageSender()
	go messageProcessor()
	go requestQueuer()
	n_systems = len(configReader.GetConfig().Servers)
	committed = toCommitChan
	requestChecker = check
	denied = failedRequestChan
	zxidCounter = ZXIDCounter{}
	epoch, count := RestoreZXIDFromDisk()
	zxidCounter.setVals(epoch, count)
	counter = &zxidCounter // for external packages to access this variable
	return
}

// Pause the sending and processing of any new requests. This is meant to be called when an election is happening.
func Pause() {
	if !requestsEnabled {
		// Requests alreaady disabled; just return
		return
	}
	logger.Warn("Pausing processing and sending of new requests")
	requestsEnabled = false
	requestsProcessingEnabled = false

	// Clear all non-request queues
	logger.Warn(fmt.Sprint("Clearing processing queue: ", queueStateToStr(&messageQueue)))
	messageQueue.clear()
	logger.Warn(fmt.Sprint("Clearing send queue: ", sendQueueStateToStr(&sendingQueue)))
	sendingQueue.clear()

	// Don't need to reset the sync state. This will be done at the Continue side.
	// Remember to reset the syncing variable on Continue in case this node is not re-elected!
}

// This function is to be called when the election has completed. This will allow ZAB nodes to send requests.
func Continue() {
	if requestsEnabled {
		// Requests already enabled; just return
		return
	}
	logger.Warn("Enabling sending of new requests")
	requestsEnabled = true
	// Enabling so it doesn't cause problems in the future. Won't affect the current epoch since only the
	// coordinator should get any request messages.
	if election.Coordinator.GetCoordinator() != configReader.GetName() {
		logger.Warn("Enabling processing of new requests")
		requestsProcessingEnabled = true
		syncing = false // If the node is not a coordinator, this should not be set.
	} else {
		SendNewLeader()
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

// Enqueue a received Zab message to the incoming processing queue
func EnqueueZabMessage(netMsg cxn.NetworkMessage) {
	var zm ZabMessage
	if err := deserialise(netMsg.Message, &zm); err != nil {
		logger.Error("Failed to unmarshal json when converting to Zab")
		return
	}
	messageQueue.enqueue(netMsg)
}

// Should only be called by the coordinator. Starts a new epoch and the syncing process.
func SendNewLeader() {
	syncZxid := zxidCounter.GetLatestZXID()
	if syncZxid == 0 {
		// This is the first leader election; don't need to sync anything.
		logger.Warn(fmt.Sprint("SendNewLeader called with ZXID = 0; don't start the syncing process."))
		syncing = false
		requestsProcessingEnabled = true
		return
	}
	prop := Proposal{}
	if !syncing {
		// Only increment the epoch if there is not an active sync session.
		epoch, count := zxidCounter.incEpoch()
		prop = Proposal{
			PropType: NewLeader,
			EpochNum: epoch,
			CountNum: count,
			Content:  uint32ToBytes(syncZxid),
		}
		currentSync = prop
	} else {
		prop = currentSync
	}
	syncing = true
	syncID++
	syncResponseTracker.Set(len(election.Addresses) - 1)
	logger.Info(fmt.Sprint("New sync with ZXID ", syncZxid, ", setting counter to ", syncResponseTracker.count))
	queueWriteProposal(prop)
	broadcastProposal(prop)
}

// Process a Zab message received from the network.
func ProcessZabMessage(netMsg cxn.NetworkMessage) {
	src := netMsg.Remote
	msgSerial := netMsg.Message

	var msg ZabMessage
	err := deserialise(msgSerial, &msg)
	if err != nil {
		logger.Debug("Not a ZabMessage")
		return
	}

	logger.Debug(fmt.Sprint("Received ZabMessage of type ", msg.ZabType.ToStr(), " from ", src))

	switch msg.ZabType {
	case Req:
		var req Request
		deserialise(msg.Content, &req)
		if requestsProcessingEnabled || req.ReqType == Sync {
			// Allow Sync requests to be processed
			processRequest(req, netMsg.Remote)
		} else {
			// if it's a write request, store the request to be processed later.
			logger.Debug(fmt.Sprint("Enqueueing message to the holding queue: ", convertZabToStr(msg)))
			holdingRequestsQueue.enqueue(netMsg)
			logger.Debug(fmt.Sprint("Holding queue state: ", queueStateToStr(&holdingRequestsQueue)))
		}
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
		logger.Error(fmt.Sprint("Received SYNC ERROR! Check if current coordinator is correct: ", election.Coordinator.GetCoordinator()))
	}

}

// Process a received proposal
func processProposal(prop Proposal, source string, originalMsg ZabMessage) {
	if election.Coordinator.GetCoordinator() != source {
		logger.Warn(fmt.Sprint("Received Proposal from ", source, ", which is not the coordinator. Ignoring..."))
		return
	}

	logger.Debug(fmt.Sprint("Received Proposal of type ", prop.PropType.ToStr(), " from ", source))
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

// Enqueues a message to the outgoing send queue.
func enqueueMessage(toSend ToSendMessage) {
	sendingQueue.enqueue(toSend)
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
			logger.Debug("Finished SYNC process")
			syncMsg := syncTrack.getStoredProposal()
			syncTrack.deactivate()
			ack := makeACK(syncMsg)
			queueSend(ack, false, source)
			select {
			case SyncFinish <- true:
			default:
			}
		}
	} else {
		// Otherwise, normal operation
		ack := makeACK(originalMsg)
		queueSend(ack, false, source)
		queueWriteProposal(prop)
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
	// First, save the proposal
	queueWriteProposal(prop)

	// Immediately commit all held proposals
	for !proposalsQueue.isEmpty() {
		poppedProposal, _ := proposalsQueue.dequeue()
		queueCommitProposal(poppedProposal)
	}

	latestZxid := bytesToUint32(prop.Content)
	currentZxid := zxidCounter.GetLatestZXID()
	// Already holding the latest proposal, just ACK and return.
	if currentZxid == latestZxid {
		logger.Debug("Received NewLeader but already up to date, ACKing the NewLeader")
		ack := makeACK(originalMsg)
		queueSend(ack, false, source)
		select {
		case SyncFinish <- true:
		default:
		}
		return
	}

	logger.Debug(fmt.Sprint("Received NewLeader of ZXID ", latestZxid, " but current ZXID is ", currentZxid, "; sending Sync request"))
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
	if prop.PropType == NewLeader {
		// ACKing the NewLeader proposal
		if !requestsProcessingEnabled && prop.CountNum == currentSync.CountNum && prop.EpochNum == currentSync.EpochNum {
			// Sync currently active
			syncResponseTracker.Decrement()
			logger.Debug(fmt.Sprint("NewLeader ACK received, decrementing counter to ", syncResponseTracker.count))
			if syncResponseTracker.count == 0 {
				// Sync done
				logger.Debug("Sync session completed, processing holding queue")
				requestsProcessingEnabled = true
				syncing = false
				unloadHoldingQueue()
			}
		} else {
			// Sync not active but ACK for NewLeader received
			// TODO decide what happenes here. I'm just gonna ignore this for now.
			logger.Warn(fmt.Sprint("Stale or unprompted ACK for NewLeader received for epoch ", prop.EpochNum, " (current epoch is ", currentSync.EpochNum, "). If the epochs are the same, sync timeout may be too short"))
			return
		}
	}
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
	name := configReader.GetName()
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
		newReq := Request{
			ReqType:   req.ReqType,
			ReqNumber: req.ReqNumber,
			Content:   updatedReq,
		}
		newReqJson, err := json.Marshal(newReq)
		if err != nil {
			logger.Fatal(fmt.Sprint("Error marshaling request ", err))
		}
		prop := Proposal{
			StateChange,
			epoch,
			count,
			newReqJson,
		}
		proposalsQueue.enqueue(prop)
		logger.Info(fmt.Sprint("Request processed successfully, broadcasting as Proposal #", zxid))
		queueWriteProposal(prop)
		broadcastProposal(prop)
	case Sync:
		// New sync server
		// Don't check coodinator status, just send to make it simple
		highestEpoch, highestCount := zxidCounter.check()
		sentzxid := bytesToUint32(req.Content)
		sentEpoch, count := decomposeZXID(sentzxid)
		logger.Info(fmt.Sprint("Got sync request with epoch ", sentEpoch, ", and count ", count))
		// if sentEpoch != currentSync.EpochNum {
		// 	logger.Debug(fmt.Sprint("Sync request is for a stale sync, current epoch is", currentSync.EpochNum, "; ignoring it."))
		// 	return nil
		// }
		// syncResponseTracker.Decrement()
		// logger.Info(fmt.Sprint("Decrementing sync counter to ", syncResponseTracker.count))

		// Sent epoch shouldn't be more than the current one.
		if sentEpoch > highestEpoch {
			replySyncRequestError(req, remoteID)
			return nil
		}

		// Need to send all proposals from the sync request number onwards.
		// If the epoch number is less, need to send epoch by epoch.
		go func() {
			for epoch := sentEpoch; epoch < highestEpoch; epoch++ {
				numProposals, err := getEpochHighestCount(epoch)
				if err != nil {
					logger.Error(fmt.Sprint("Failed to get ZXID from proposal store:", err))
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
	serial, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
		return
	}
	msg := ZabMessage{
		Req,
		serial,
	}
	queueSend(msg, false, election.Coordinator.GetCoordinator())
}

func queueSend(zabMsg ZabMessage, isBroadcast bool, remote string) {
	if configReader.GetName() == remote {
		// If the message is directed to itself, directly enqueue the message instead of sending it through the network.
		content, err := json.Marshal(zabMsg)
		if err != nil {
			logger.Fatal("Failed to marshal ZAB message!")
		}
		nm := cxn.NetworkMessage{
			Remote:  configReader.GetName(),
			Type:    cxn.ZAB,
			Message: content,
		}
		EnqueueZabMessage(nm)
		return
	}
	toSendMsg := ToSendMessage{
		msg:       zabMsg,
		broadcast: isBroadcast,
		target:    remote,
	}
	logger.Debug(fmt.Sprint("Enqueueing send message: ", convertZabToStr(zabMsg)))
	enqueueMessage(toSendMsg)
	logger.Debug(fmt.Sprint("Send queue state: ", sendQueueStateToStr(&sendingQueue)))
}

func queueWriteProposal(prop Proposal) {
	newProposalChan <- prop
}

func queueCommitProposal(prop Proposal) {
	var committedRequest Request
	err := deserialise(prop.Content, &committedRequest)
	if err != nil {
		logger.Error(fmt.Sprint("Failed to convert committed proposal content to request: ", err))
	}
	toCommitChan <- committedRequest
}

// Send a Zab message over the network.
func sendZabMessage(dest string, msg ZabMessage) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	err = cxn.SendMessage(cxn.NetworkMessage{Remote: dest, Type: cxn.ZAB, Message: serial})
	if err != nil {
		logger.Error(fmt.Sprint("Could not send message to ", dest, ":", err))
	}
}

// Broadcast a Zab message over the network.
func broadcastZabMessage(msg ZabMessage) {
	serial, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
		return
	}
	cxn.CustomBroadcast(election.Addresses, serial, cxn.ZAB)
}
