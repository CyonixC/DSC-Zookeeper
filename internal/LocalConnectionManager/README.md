# Local Connection Manager
This package is meant to be used to simulate (but not totally match) the API of the actual connection manager.

This is NOT meant to be used with Docker! Only use this for testing on a **single machine**.

## Usage
Import this file in your code:
```go
import (
    // other packages...
    "local/zookeeper/internal/LocalConnectionManager"
)
```

To initialise, call the `Init()` function. This returns a single channel, the receive channel. Newly received messages will come in on this channel.

```go
localIP = "192.168.10.1"
recv := connectionManager.Init(localIP)
```

To send a message, call the `SendMessage()` function:
```go
var msg []byte
var failed chan string
localIP = "192.168.10.1"
remoteAddr := "192.168.0.2"
err := connectionManager.SendMessage(NetworkMessage{remoteAddr, msg}, localIP, failed)
```

To broadcast a message, call the `Broadcast()` function:
```go
var msg []byte
var failed chan string
localIP = "192.168.10.1"
connectionManager.Broadcast(msg, localIP, failed)
```

Both `SendMessage` and `Broadcast` take a "failed" channel argument which outputs IP addresses machines which messages failed to send to.

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

