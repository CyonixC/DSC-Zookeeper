package proposals

import (
	"fmt"
	"local/zookeeper/internal/election"
	"local/zookeeper/internal/logger"
	"time"
)

// This file defines definitions for goroutines meant to be run constantly in
// the background. The functions are defined for tasks which must be completed
// sequentially.

type ToSendMessage struct {
	msg       ZabMessage
	broadcast bool
	target    string
}

// This is meant to manage writing of proposals to the disk.
func proposalWriter(newProposalChan chan Proposal) {
	for prop := range newProposalChan {
		SaveProposal(prop)
	}
}

func messageSender() {
	for {
		for sendingQueue.isEmpty() {
			continue
		}
		toSend, _ := sendingQueue.dequeue()
		if toSend.broadcast {
			logger.Debug(fmt.Sprint("Broadcasting message: ", convertZabToStr(toSend.msg)))
			broadcastZabMessage(toSend.msg)
			logger.Debug(fmt.Sprint("Broadcasted message, ", convertZabToStr(toSend.msg)))
		} else {
			logger.Debug(fmt.Sprint("Sending message: ", convertZabToStr(toSend.msg), " to ", toSend.target))
			sendZabMessage(toSend.target, toSend.msg)
			logger.Debug(fmt.Sprint("Sent message: ", convertZabToStr(toSend.msg), " to ", toSend.target))
		}
		logger.Debug(fmt.Sprint("Send queue state: ", sendQueueStateToStr(&sendingQueue)))
	}
}

func messageProcessor() {
	for {
		for messageQueue.isEmpty() {
			continue
		}
		msg, _ := messageQueue.dequeue()
		logger.Debug(fmt.Sprint("Processing message: ", convertMessageToStr(msg)))
		logger.Debug(fmt.Sprint("Incoming queue state: ", queueStateToStr(&messageQueue)))
		ProcessZabMessage(msg)
		logger.Debug(fmt.Sprint("Processed ZAB message from ", msg.Remote))
	}
}

func requestQueuer() {
	for {
		for requestQueue.isEmpty() || !requestsEnabled {
			continue
		}
		req, _ := requestQueue.dequeue()
		logger.Info(fmt.Sprint("Queued request from outgoing requests for sending: ", convertZabToStr(req)))
		logger.Info(fmt.Sprint("Outgoing request queue state: ", zabQueueStateToStr(&requestQueue)))
		queueSend(req, false, election.Coordinator.GetCoordinator())
	}
}

func unloadHoldingQueue() {
	logger.Info(fmt.Sprint("Unloading the holding queue. Queue state: ", queueStateToStr(&holdingRequestsQueue)))
	for !holdingRequestsQueue.isEmpty() {
		req, _ := holdingRequestsQueue.dequeue()
		EnqueueZabMessage(req)
	}
}

// TODO check if I used this anywhere
func syncTimeout(id int) {
	time.Sleep(syncResponseTimeout)
	if id == syncID {
		logger.Info("Sync session timed out. Processing requests queue")
		requestsProcessingEnabled = true
		syncing = false
		unloadHoldingQueue()
	}
}
