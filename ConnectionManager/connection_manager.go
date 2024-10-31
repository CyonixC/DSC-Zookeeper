package connectionManager

// Contains functions for sending and receiving TCP messages.

import (
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"sync"
	"time"
)

var ipToConnection = SafeConnectionMap{
	connMap: make(map[string]net.Conn),
}

type NetworkMessage struct {
	Remote  string
	Message []byte
}

type SafeConnectionMap struct {
	sync.RWMutex
	connMap map[string]net.Conn
}

func (smap *SafeConnectionMap) store(key net.Addr, val net.Conn) {
	smap.Lock()
	defer smap.Unlock()
	smap.connMap[key.String()] = val
}

func (smap *SafeConnectionMap) load(key net.Addr) (net.Conn, bool) {
	smap.RLock()
	defer smap.RUnlock()
	val, ok := smap.connMap[key.String()]
	return val, ok
}

func (smap *SafeConnectionMap) loadOrStore(key net.Addr, val net.Conn) (net.Conn, bool) {
	oldval, ok := smap.load(key)
	if !ok {
		smap.store(key, val)
		return val, false
	} else {
		return oldval, true
	}
}

func startTCPListening(newConnChan chan net.Conn) (int, error) {
	// Start listening on a TCP socket.

	serverSocketAddr := fmt.Sprintf(":%d", portNum)
	ln, err := net.Listen("tcp", serverSocketAddr)
	if err != nil {
		return 0, err
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Could not accept tcp connection:", err)
		}

		newConnChan <- conn
	}
}

func startReceiving(recv_channel chan NetworkMessage, connection net.Conn) {
	defer connection.Close()
	buffer := make([]byte, 1024)
	for {
		// Read binary data into the buffer
		_, err := connection.Read(buffer)
		if err != nil {
			// If EOF, the connection was closed by the remote host
			if err.Error() == "EOF" {
				fmt.Println("Connection closed by remote host.")
				break
			}
			fmt.Println("Error reading from connection:", err)
			break
		}

		ret := make([]byte, 1024)
		copy(ret, buffer)
		go func() { recv_channel <- NetworkMessage{connection.RemoteAddr().String(), ret} }()
	}
}

func makeSocketAddrList() ([]net.Addr, error) {
	var allAddresses []net.Addr
	if slices.Contains(ip_list, "localhost") || slices.Contains(ip_list, "127.0.0.1") {
		// using localhost setting
		tcpPortStr := fmt.Sprintf("localhost:%d", portNum)
		tcpPort, err := net.ResolveTCPAddr("tcp", tcpPortStr)
		if err != nil {
			return nil, err
		}
		allAddresses = append(allAddresses, tcpPort)
	} else {
		// using different remote hosts
		for _, addr := range ip_list {
			tcpPortStr := fmt.Sprintf("%s:%d", addr, portNum)
			tcpPort, err := net.ResolveTCPAddr("tcp", tcpPortStr)
			if err != nil {
				return nil, err
			}
			allAddresses = append(allAddresses, tcpPort)
		}
	}
	return allAddresses, nil
}

func zipAndMerge[T any, L any](ins []chan T, labels []L) (chan struct {
	msg   T
	label L
}, error) {
	// Fan-in pattern
	if len(labels) != len(ins) {
		return nil, errors.New("length of labels slice in merge should be the same as the length of the channel slice")
	}
	out := make(chan struct {
		msg   T
		label L
	})
	output := func(in <-chan T, label L) {
		for msg := range in {
			out <- struct {
				msg   T
				label L
			}{msg, label}
		}
	}
	for i, in := range ins {
		go output(in, labels[i])
	}
	return out, nil
}

func connectToSystemServers(socketAddresses []net.Addr, newConnChan chan net.Conn) {
	// Attempt to establish connections to all known machines in the system.
	successChannels := make([]chan net.Conn, len(socketAddresses))
	for i := range successChannels {
		successChannels[i] = make(chan net.Conn)
	}
	for i, sock := range socketAddresses {
		go attemptConnection(sock, successChannels[i])
	}
	timer := time.NewTimer(time.Duration(tcpEstablishTimeoutSeconds) * time.Second)
	completed := 0
	results, err := zipAndMerge(successChannels, socketAddresses)
	if err != nil {
		log.Fatal("In connectToSystemServers", err)
	}
	for completed < len(socketAddresses) {
		select {
		case res := <-results:
			if res.msg == nil {
				// Connection attempt was unsuccessful
				go func() {
					time.Sleep(time.Duration(tcpRetryConnectionTimeoutSeconds) * time.Second)
					attemptConnection(res.label, successChannels[slices.Index(socketAddresses, res.label)])
				}()
			} else {
				// Connection attempt was successful
				completed++
				go func() { newConnChan <- res.msg }()
			}
		case <-timer.C:
			// Give up attempting to connect to the servers and just return
			close(results)
			return
		}
	}
}

func attemptConnection(socketAddress net.Addr, successChan chan net.Conn) {
	// Attempt to connect to the given TCP socket.
	conn, err := net.Dial("tcp", socketAddress.String())

	// Channel may be closed; in that case, ignore the panic.
	defer func() { recover() }()

	if err != nil {
		// Failed to connect, return nil
		successChan <- nil
		return
	}
	successChan <- conn
}

func Init() (receiveChannel chan NetworkMessage) {
	// Initialise all the things needed to maintain and track connections between nodes. This includes:
	// Set up a TCP listener on a specified port (portNum in env.go)
	// Repeatedly ping the known machines in the network until connections are established with all

	// New connections will arrive on this channel
	newConnChan := make(chan net.Conn)

	// This channel is used to receive from external nodes
	receiveChannel = make(chan NetworkMessage)

	// Initialise TCP listener
	go startTCPListening(newConnChan)

	// Initialise TCP pinger
	socketAddresses, err := makeSocketAddrList()
	if err != nil {
		log.Fatal("Could not process node IP addresses; check the configuration file", err)
	}
	go connectToSystemServers(socketAddresses, newConnChan)
	go connectionManager(receiveChannel, newConnChan)
	return
}

func SendMessage(toSend NetworkMessage) error {
	// Find the right connection and send the message.
	// If the connection does not exist, attempt to establish it.
	// This is a blocking call!
	remote, err := net.ResolveIPAddr("ip", toSend.Remote)
	if err != nil {
		return err
	}
	sendConnection, ok := ipToConnection.load(remote)
	if ok {
		_, err := sendConnection.Write(toSend.Message)
		if err != nil {
			return err
		}
	} else {
		// We don't have an existing connection with this machine.
		tempChan := make(chan net.Conn)
		go attemptConnection(remote, tempChan)
		newConnection := <-tempChan
		if newConnection != nil {
			ipToConnection.store(newConnection.RemoteAddr(), newConnection)
			_, err := newConnection.Write(toSend.Message)
			if err != nil {
				return err
			}
		} else {
			// Failed to establish new connection
			return errors.New("could not establish TCP connection to target")
		}
	}
	return nil
}

func Broadcast(toSend []byte) {
	ipToConnection.RLock()
	defer ipToConnection.RUnlock()
	for addr := range ipToConnection.connMap {
		go SendMessage(NetworkMessage{addr, toSend})
	}
}

func connectionManager(receiveChannel chan NetworkMessage, newConnectionChan chan net.Conn) {
	// Contains a mapping between network addresses and connections, and primarily manages the sending of messages.
	for newConnection := range newConnectionChan {
		// A new connection has come in. If we already have a connection to this node, close it.
		_, ok := ipToConnection.load(newConnection.RemoteAddr())
		if ok {
			// We already have an existing connection with this machine.
			newConnection.Close()
		} else {
			// Store the new connection and start receiving
			go startReceiving(receiveChannel, newConnection)
			ipToConnection.store(newConnection.RemoteAddr(), newConnection)
		}
	}
}
