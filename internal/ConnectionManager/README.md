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

### Message format
The format of the send and received messages is the `NetworkMessage` struct:
```go
type NetworkMessage struct {
	Remote  string
	Message []byte
}
```

`Remote` is the name of the node. With Docker, this is the name of the container. Messages to be sent / received should be serialised / deserialised accordingly.

### Initialisation
To initialise, call the `Init()` function.

```go
func Init() (receiveChannel chan NetworkMessage, failedSends chan string)
```

This returns two channels: the receive channel, and the failure channel. 
1. Receive: Newly received messages will come in on this channel.
2. Failure: The ID of any nodes that the current node failed to send to comes in on this channel.

Both channels should be watched for incoming messages.

For most reliable performance, put a wait before and after the `Init()` call.

Example:
```go
time.Sleep(time.Second)
recv, failed := connectionManager.Init()
time.Sleep(time.Second)
```

### Message sending
To send a message, call the `SendMessage()` function:

```go
func SendMessage(toSend NetworkMessage) error
```

The target of the message should be placed in the network message struct in the `Remote` field.

If an existing connection has not been established with the remote machine, this function will attempt to establish a connection. In this case, if it fails to create the connection, THE FAILEDSENDS CHANNEL WILL NOT OUTPUT AN ERROR. If you want to attempt a connection to a new machine and get back a success/failure message, check the if the error returned by this function is `nil`.

```go
var msg []byte
remoteID := "server1"
netMsg := connectionManager.NetworkMessage{
    Remote: remoteID,
    Message: msg,
}
connectionManager.SendMessage(netMsg)
```

To broadcast a message, call the `Broadcast()` function. Note that this broadcasts to ALL KNOWN MACHINES! If you just want to broadcast to known servers, see ServerBroadcast below.
Note that unlike `SendMessage`, this takes a regular `[]byte` array, not a `NetworkMessage`.
```go
var msg []byte
connectionManager.Broadcast(msg)
```

To broadcast to all known servers, use the `ServerBroadcast()` function. Currently, it just checks if the machine name contains "server" as the prefix.
```go
var msg []byte
connectionManager.ServerBroadcast(msg)
```