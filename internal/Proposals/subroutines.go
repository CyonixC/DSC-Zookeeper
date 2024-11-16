package proposals

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
			broadcastZabMessage(toSend.msg)
		} else {
			sendZabMessage(toSend.target, toSend.msg)
		}
	}
}
