package connectionManager

import "net"

type NetMessageType int

const (
	ZAB NetMessageType = iota
	HEARTBEAT
	ELECTION
	CLIENTMSG
)

type NetworkMessage struct {
	Remote  string
	Type    NetMessageType
	Message []byte
}

type NamedConnection struct {
	Remote     string
	Connection net.Conn
}
