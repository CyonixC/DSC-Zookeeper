package proposals

// This file contains struct definitions for handling all messages exchanged in the Zab protocol.

// This structure encapsulates all Zab messages.
type ZabMessageType int
type ZabMessage struct {
	ZabType ZabMessageType
	Content []byte
}

const (
	Req ZabMessageType = iota
	Prop
	ACK
	Err
)

// Proposal type - message sent from coordinator to non-coordinators
type ProposalType uint8
type Proposal struct {
	PropType ProposalType
	EpochNum uint16
	CountNum uint16
	Content  []byte
}

const (
	Commit ProposalType = iota
	StateChange
	NewLeader
)

// Request type - message sent from a non-coordinator to the coordinator
type RequestType int
type Request struct {
	ReqType RequestType
	Content []byte
}

const (
	Sync RequestType = iota
	Write
)

type Deserialisable interface {
	ZabMessage | Proposal | Request
}

func (pt ProposalType) ToStr() string {
	switch pt {
	case Commit:
		return "Commit"
	case StateChange:
		return "StateChange"
	case NewLeader:
		return "NewLeader"
	}
	return "unknown"
}
