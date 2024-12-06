package connectionManager

import "net"

type NetMessageType int

const (
	ZAB NetMessageType = iota
	HEARTBEAT
	ELECTION
	CLIENTMSG
)

func (n NetMessageType) ToStr() string {
	switch n {
	case ZAB:
		return "ZAB"
	case HEARTBEAT:
		return "HEARTBEAT"
	case ELECTION:
		return "ELECTION"
	case CLIENTMSG:
		return "CLIENT_MSG"
	default:
		return ""
	}
}

type NetworkMessage struct {
	Remote  string
	Type    NetMessageType
	Message []byte
}

type NamedConnection struct {
	Remote     string
	Connection net.Conn
}
