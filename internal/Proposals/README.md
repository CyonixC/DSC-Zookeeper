# Zab proposals system
This package implements the proposals portion of the Zab protocol.

## Structure
The main code is in `proposals.go`, with supplementary code in the other files containing helper structures and type definitions.

## Usage
Generally the process is as follows:
1. Use the `ConnectionManager`'s `Init()` function to get the network channel.
2. Use `Init()` to connect the proposals process with the network channel, initialise the proposals background goroutines, and get the error / receive channels.
3. Start monitoring the receive channels for incoming messages.
4. Send messages to the coordinator node with `Send`

For details on Step 1, refer to the `ConnectionManager` package.

### 1. Initialise
Use `Init()` to initialise the proposal handler. 

```go
type checkFunction func([]byte) ([]byte, error)
func Init(networkChannel chan cxn.NetworkMessage, check checkFunction) (committed chan []byte, denied chan Request)
```
- `networkChannel` - incoming `NetworkMessage`s from the network should arrive on this channel.
- `check` - this function is used to check potential updates to the data state. It should take in a proposed commit. The return `[]byte` value should be either a commit, which will be used to update the system state, or an error, which will be returned to the sender of the request. The `error` return value is **ONLY USED FOR LOGGING** and will not be returned to the sender!

**Returns**
- `committed` will send any proposals which are to be committed. These are ones that have already been checked by the coordinator. Any proposal received on this should be committed sequentially and immediately.
- `denied` will send any proposals which failed the check at the coordinator. The commit contained inside is the one returned by the supplied `check` function on check fail.

**Example**
```go
import proposals {
	proposals "local/zookeeper/internal/Proposals"
	connectionManager "local/zookeeper/internal/ConnectionManager"
}
// Check function
func acceptAll(m []byte) ([]byte, error)  { return m, nil }
// Network channel
recv_channel, _ := connectionManager.Init()

commitChan, denied := proposals.Init(recv_channel, acceptAll)
```

### 2. Send requests
To send a Zab request, use the `SendWriteRequest` function. 

```go
func SendWriteRequest(content []byte, requestNum int)
```
- `content` - the serialised commit request message.
- `requestNum` - the serial number that should be associated with this request. This is to help associate requests with returned errors. For a single session, the IDs of all its requests should be unique to prevent errors from being associated with the wrong request.
