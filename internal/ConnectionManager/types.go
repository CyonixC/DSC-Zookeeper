package connectionManager

import "net"

type NetworkMessage struct {
	Remote  string
	Message []byte
}

type NamedConnection struct {
	Remote     string
	Connection net.Conn
}
