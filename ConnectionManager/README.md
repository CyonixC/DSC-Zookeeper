# Connection Manager
This package is meant to be used to easily manage the establishing of TCP connections and sending of messages between nodes.

## Usage
Import this file in your code:
```go
import (
    // other packages...
    "zookeeper/ConnectionManager"
)
```

To initialise, call the `Init()` function. This returns a single channel, the receive channel. Newly received messages will come in on this channel.

```go
recv := connectionManager.Init()
```

To send a message, call the `SendMessage()` function:
```go
var msg []byte
remoteAddr := "192.168.0.1"
err := connectionManager.SendMessage(NetworkMessage{remoteAddr, msg})
```

To broadcast a message, call the `Broadcast()` function:
```go
var msg []byte
connectionManager.Broadcast(msg)
```

## Message format
The format of the send and received messages is the `NetworkMessage` struct:
```go
type NetworkMessage struct {
	Remote  string
	Message []byte
}
```

Messages to be sent / received should be serialised / deserialised accordingly.

Note that the `Broadcast()` function does not require the message to be wrapped in the struct.
