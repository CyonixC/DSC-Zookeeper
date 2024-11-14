# Connection Manager
This package is meant to be used to easily manage the establishing of TCP connections and sending of messages between nodes.

## Usage
Import this file in your code:
```go
import (
    // other packages...
    "local/zookeeper/internal/ConnectionManager"
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

To broadcast a message, call the `Broadcast()` function. Note that this broadcasts to ALL KNOWN MACHINES! If you just want to broadcast to known servers, see ServerBroadcast below.
```go
var msg []byte
connectionManager.Broadcast(msg)
```

To broadcast to all known servers, use the `ServerBroadcast()` function. Currently, it just checks if the machine name contains "server" as the prefix.
```go
var msg []byte
connectionManager.ServerBroadcast(msg)
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
