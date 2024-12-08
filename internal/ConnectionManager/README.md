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

### Initialisation
To initialise, call the `Init()` function.

```go
func Init() (receiveChannel chan NetworkMessage, failedSends chan string)
```

This returns two channels: the receive channel, and the failure channel. 
1. Receive: Newly received messages will come in on this channel.
2. Failure: The ID of any nodes that the current node failed to send to comes in on this channel.

Both channels should be watched for incoming messages.

Example:
```go
recv, failed := connectionManager.Init()
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
func Broadcast(toSend []byte, msgType NetMessageType)
```

To broadcast to all known servers, use the `ServerBroadcast()` function.
```go
var msg []byte
func ServerBroadcast(toSend []byte, msgType NetMessageType)
```

To broadcast to a specific set of nodes, use the `CustomBroadcast()` function.
```go
var msg []byte
func CustomBroadcast(dests []string, toSend []byte, msgType NetMessageType)
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

## Internals
The ConnectionManager package depends on two `ConnectionMap`s:
1. Read map
2. Write map

The Read map contains a mapping from node names to an established TCP connection to that node. This connection will only ever be used to receive messages from the other node.

The Write map contains a mapping from node names to an established TCP connection to that node. This connection will only ever be used to write messages to the other side.

New connections are added to the maps by two corresponding goroutines:
1. The Read map is maintained by the `readConnectionManager`
2. The Write map is maintained by the `writeConnectionManager`

Both managers watch a corresponding Go channel which new channels are sent on: the READ channel is newReadChan, and the WRITE channel is newWriteChan.

When the manager returns (due to an interrupt, e.g.), it also closes all connections on its corresponding channel.

### Connection failure
READ connections can detect if a connection has failed automatically, if the read returns an EOF error. If a READ connection fails, it is closed silently.

WRITE connections are monitored for failure with the `monitorConnection()` function, which constantly attempts to read 1 byte and checks if it is the EOF error. If a WRITE connection fails, it is closed, removed from the Write map, and the name of the node whose connection failed is reported on the external FailedSends channel (one of the channels returned by the `Init()` function).

### Connection establishment
The establishment of new connections is handled by the `attemptConnection()` function. This function sends TCP connection requests in intervals of `tcpRetryConnectionTimeout`, timing out in `tcpEstablishConnectionTimeout`.
