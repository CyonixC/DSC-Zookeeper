# Zab proposals system
This package implements the proposals portion of the Zab protocol.

## Structure
The main code is in `proposals.go`, with supplementary code in the other files containing helper structures and type definitions.

## Usage
There are two main interactions that need to be handled: sending of Zab requests, and receiving of Zab messages.

To handle Zab messages, simply create a goroutine with `ProcessZabMessage` and pass it in.

```go
func receiveMsg(recv_channel chan cxn.NetworkMessage, failedSends chan string, selfIP net.Addr) {
	for msg := range recv_channel {
		go proposals.ProcessZabMessage(msg, failedSends, selfIP)
	}
}
```

This function additionally takes in the current machine's IP address and a "failed sending" channel where the IP addresses of any machines it fails to send to (e.g. for ACKs) is returned.


To send a Zab request, use the `SendWriteRequest` function. 

```go
proposals.SendWriteRequest(newVar, failedSends, selfIP)
```

This function also takes in the current machine's IP address and a "failed sending" channel where the IP addresses of any machines it fails to send to is returned.

## TODO
Currently the Proposals queue is still held in memory, not on disk. Need to make committed proposals be written to disk.

Yet to implement the SYNC protocol for when a new coordinator is elected and we need nodes to sync proposals with each other.