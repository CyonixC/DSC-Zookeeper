package proposals

import (
	"fmt"
	"local/zookeeper/internal/logger"
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

func messageSender(toSendChan chan ToSendMessage) {
	for toSend := range toSendChan {
		if toSend.broadcast {
			logger.Debug(fmt.Sprint("Broadcasting message"))
			broadcastZabMessage(toSend.msg)
			logger.Debug(fmt.Sprint("Broadcasted message"))
		} else {
			logger.Debug(fmt.Sprint("Sending message to ", toSend.target))
			sendZabMessage(toSend.target, toSend.msg)
			logger.Debug(fmt.Sprint("Sent message to ", toSend.target))
		}
	}
}

func messageProcessor() {
	for {
		for messageQueue.isEmpty() || !enabled {
			continue
		}
		msg, _ := messageQueue.dequeue()
		ProcessZabMessage(msg)
	}
}
