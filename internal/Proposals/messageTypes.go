package proposals

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"reflect"
)

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
	SyncErr
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

// Convert enum to string, for debugging
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

// Convert enum to string, for debugging
func (pt ZabMessageType) ToStr() string {
	switch pt {
	case Req:
		return "Request"
	case Prop:
		return "Proposal"
	case ACK:
		return "Acknowledge"
	case Err:
		return "Error"
	case SyncErr:
		return "Sync Error"
	}
	return "unknown"
}

// Request type - message sent from a non-coordinator to the coordinator
type RequestType int
type Request struct {
	ReqType   RequestType
	ReqNumber int
	Content   []byte
}

const (
	Sync RequestType = iota
	Write
)

type Deserialisable interface {
	ZabMessage | Proposal | Request
}

// Un-JSON-ify a JSON data slice into a Zab message type.
func deserialise[m Deserialisable](serialised []byte, msgPtr *m) {
	err := json.Unmarshal(serialised, msgPtr)
	if err != nil {
		log.Fatal("Could not convert ", reflect.TypeOf(msgPtr), " from bytes: ", err)
	}
}

// Convert the epoch and count numbers to a single zxid
func getZXIDAsInt(epoch uint16, count uint16) uint32 {
	return (uint32(epoch) << 16) | uint32(count)
}

// Convert a single zxid to epoch and count
func decomposeZXID(zxid uint32) (epoch uint16, count uint16) {
	epoch = uint16(zxid >> 16)
	count = uint16(zxid & 0xFFFF)
	return
}

func bytesToUint32(bytes []byte) uint32 {
	return binary.NativeEndian.Uint32(bytes)
}
func uint32ToBytes(num uint32) []byte {
	bytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(bytes, num)
	return bytes
}
