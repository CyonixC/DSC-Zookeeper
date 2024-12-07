package connectionManager

// Contains functions for sending and receiving TCP messages.
// For a single node, two connections are maintained, for read and write.
// The write channei is created when this node actively tries to connect to the other node.
// The read channel is created when the other node tries to connect to this node.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	"local/zookeeper/internal/logger"
	"log"
	"net"
	"os"
	"slices"
	"sync"
	"time"
)

var ipToConnectionRead = SafeConnectionMap{
	connMap: make(map[string]net.Conn),
}
var ipToConnectionWrite = SafeConnectionMap{
	connMap: make(map[string]net.Conn),
}
var newReadChan chan net.Conn
var newWriteChan chan NamedConnection
var removeWriteChan chan NamedConnection
var failedSendsChan chan string

// Initialise all the things needed to maintain and track connections between nodes. This includes:
// Set up a TCP listener on a specified port (portNum in config.go)
// Repeatedly ping the known machines in the network until connections are established with all
func Init() (receiveChannel chan NetworkMessage, failedSends chan string) {

	// New read connections will arrive on this channel
	newReadChan = make(chan net.Conn, 10)

	// Newly established write connections will be sent here
	newWriteChan = make(chan NamedConnection, 10)

	// This channel is used to receive from external nodes
	receiveChannel = make(chan NetworkMessage, 100)

	// The ID of any machine which this node failed to send to is sent on this channel.
	// This failure should be handled by external code.
	failedSends = make(chan string, 10)

	// Any failed connections are sent to this channel.
	removeWriteChan = make(chan NamedConnection, 10)

	// Initialise TCP listener
	go startTCPListening(newReadChan)

	// Start background goroutines
	go readConnectionManager(receiveChannel, newReadChan)
	go writeConnectionManager(newWriteChan)
	go removeConnectionManager(failedSends)

	serverNames := configReader.GetConfig().Servers
	connectToSystemServers(serverNames)
	return
}

// Find the right connection and send the message.
// If the connection does not exist, attempt to establish it.
// This is a blocking call!
func SendMessage(toSend NetworkMessage) error {
	logger.Debug(fmt.Sprint("Attempting to send message to ", toSend.Remote, "..."))
	sendConnection, ok := ipToConnectionWrite.load(toSend.Remote)
	remote := toSend.Remote
	myName := configReader.GetName()
	toSend.Remote = myName
	msg, err := json.Marshal(toSend)
	if err != nil {
		logger.Fatal(fmt.Sprint("Could not marshal message to JSON: ", toSend))
	}
	msg = append(msg, '\n')
	if ok {
		deadline := time.Now().Add(tcpWriteTimeout)
		sendConnection.SetDeadline(deadline)
		_, err = sendConnection.Write(msg)
		logger.Debug(fmt.Sprint("Sending message to ", remote, "..."))
		if err != nil {
			logger.Error(fmt.Sprint("Error sending message to ", remote))
			removeWriteChan <- NamedConnection{
				Remote:     remote,
				Connection: sendConnection,
			}
			return err
		}
	} else {
		logger.Debug(fmt.Sprint("Attempting to send to ", remote, " when a connection does not exist"))
		// We don't have an existing connection with this machine.
		tempChan := make(chan NamedConnection)
		go attemptConnection(remote, tempChan)
		newConnection := <-tempChan
		if newConnection.Connection != nil {
			logger.Debug(fmt.Sprint("Connection attempt to ", remote, " was successful"))
			newWriteChan <- newConnection
			newConnection.Connection.SetDeadline(time.Now().Add(tcpWriteTimeout))
			_, err = newConnection.Connection.Write(msg)
			logger.Debug(fmt.Sprint("Sending message to ", remote, "..."))
			if err != nil {
				logger.Error(fmt.Sprint("Error sending message to ", remote))
				removeWriteChan <- newConnection
				return err
			}
		} else {
			// Failed to establish new connection
			logger.Error(fmt.Sprint("Could not establish connection to ", remote))
			// failedSendsChan <- remote
			return errors.New("could not establish TCP connection to target")
		}
	}
	return nil
}

// Broadcast to all known machines.
func Broadcast(toSend []byte, msgType NetMessageType) {
	logger.Debug("Broadcasting message...")
	ipToConnectionWrite.RLock()
	connMap := ipToConnectionWrite.connMap
	ipToConnectionWrite.RUnlock()
	var wg sync.WaitGroup
	for addr := range connMap {
		if addr == configReader.GetName() {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := SendMessage(NetworkMessage{addr, msgType, toSend})
			if err != nil {
				logger.Error(fmt.Sprint("Error in broadcast:", err))
			}
		}()
	}
	wg.Wait()
}

func ServerBroadcast(toSend []byte, msgType NetMessageType) {
	logger.Debug("Broadcasting message to servers...")
	var wg sync.WaitGroup
	for _, name := range configReader.GetConfig().Servers {
		if name == configReader.GetName() {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := SendMessage(NetworkMessage{name, msgType, toSend})
			if err != nil {
				logger.Error(fmt.Sprint("Error in server broadcast:", err))
			}
		}()
	}
	wg.Wait()
}

func CustomBroadcast(dests []string, toSend []byte, msgType NetMessageType) {
	logger.Debug(fmt.Sprint("Broadcasting message to: ", dests))
	var wg sync.WaitGroup
	for _, name := range dests {
		if name == configReader.GetName() {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := SendMessage(NetworkMessage{name, msgType, toSend})
			if err != nil {
				logger.Error(fmt.Sprint("Error in server broadcast:", err))
			}
		}()
	}
	wg.Wait()
}

// Start listening on a TCP socket. The port number is in config.go.
func startTCPListening(newConnChan chan net.Conn) (int, error) {

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

func monitorConnection(connection net.Conn, id string) {
	for {
		one := make([]byte, 1)
		connection.SetDeadline(time.Time{})
		_, err := connection.Read(one)
		if err != nil && err.Error() == "EOF" {
			logger.Error(fmt.Sprint("Closed write connection detected to ", id))
			removeWriteChan <- NamedConnection{
				Remote:     id,
				Connection: connection,
			}
			return
		}
	}
}

// Start receiving data on the given connection. Any data is sent to recv_channel.
func startReceiving(recv_channel chan NetworkMessage, connection net.Conn) {
	defer func() {
		logger.Warn(fmt.Sprint("Read connection to ", connection.RemoteAddr().String(), " closing"))
		connection.Close()
	}()
	reader := bufio.NewReader(connection)
	for {
		// Read binary data into the buffer
		connection.SetDeadline(time.Time{})
		msg, err := reader.ReadBytes('\n')
		if err != nil {
			// If EOF, the connection was closed by the remote host
			if err.Error() == "EOF" {
				logger.Error(fmt.Sprint("Connection with ", connection.RemoteAddr(), " closed by remote host."))
				break
			}
			fmt.Println("Error reading from connection:", err)
			break
		}

		ret := NetworkMessage{}
		err = json.Unmarshal(msg[:len(msg)-1], &ret)
		if err != nil {
			logger.Fatal(fmt.Sprint("Could not convert received message to JSON: ", string(msg[:len(msg)-1])))
		}
		logger.Debug(fmt.Sprint("Received message from ", connection.RemoteAddr().String(), " of type ", ret.Type.ToStr()))
		recv_channel <- ret
	}
}

// Unused
func makeSocketAddrList() ([]net.Addr, error) {
	var allAddresses []net.Addr
	ip_list := configReader.GetConfig().Servers
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

// Attempt to establish connections to all known machines in the system.
func connectToSystemServers(serverNames []string) {
	// Successful connections will be sent here
	successChan := make(chan NamedConnection)

	myName := os.Getenv("NAME")
	role := os.Getenv("MODE")
	var establishCount int
	if role == "Server" {
		establishCount = len(serverNames) - 1
	} else {
		establishCount = len(serverNames)
	}

	// For each server, attempt to make connection
	for _, serverName := range serverNames {
		if serverName == myName {
			continue
		}
		go attemptConnection(serverName, successChan)
	}
	timer := time.NewTimer(tcpEstablishTimeout)
	completed := 0
	for completed < establishCount {
		select {
		case res := <-successChan:
			if res.Connection == nil {
				// Connection attempt was unsuccessful
				go func() {
					attemptConnection(res.Remote, successChan)
				}()
			} else {
				// Connection attempt was successful
				completed++
				go func() { newWriteChan <- res }()
			}
		case <-timer.C:
			// Give up attempting to connect to the servers and just return
			close(successChan)
			return
		}
	}
}

// Attempt to connect to the given server.
func attemptConnection(serverName string, successChan chan NamedConnection) bool {
	logger.Debug(fmt.Sprint("Attempting to connect to ", serverName))
	connectionDeadline := time.Now().Add(tcpEstablishTimeout)

	for !time.Now().After(connectionDeadline) {
		// Dial with timeout
		timeout := tcpRetryConnectionTimeout
		deadline := time.Now().Add(timeout)
		conn, err := net.DialTimeout("tcp", serverName+":8080", timeout)

		if err != nil {
			// Failed to connect, sleep until deadline and try again
			if connectionDeadline.Before(deadline) {
				// If the overall connection deadline is earlier than the next deadline, just break
				break
			} else {
				// Otherwise, sleep until the next deadline.
				time.Sleep(time.Until(deadline))
			}
		} else {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Warn(fmt.Sprint("Succeeded but channel is closed: ", r))
					}
				}()
				successChan <- NamedConnection{
					Remote:     serverName,
					Connection: conn,
				}
			}()
			logger.Debug(fmt.Sprint("Successfully connected to ", serverName))
			go monitorConnection(conn, serverName)
			return true
		}

	}
	// Channel may be closed; in that case, ignore the panic.
	defer func() {
		if r := recover(); r != nil {
			logger.Warn(fmt.Sprint("Connection failed with a panic: ", r))
		}
	}()
	successChan <- NamedConnection{
		Remote:     serverName,
		Connection: nil,
	}
	return false
}

// Runs in the background to log new write connections. If a new one is established while there
// already exists another connection to the same server, the old one is deleted and replaced with
// the new one.
func writeConnectionManager(newConnectionChan chan NamedConnection) {
	defer func() {
		for _, conn := range ipToConnectionWrite.connMap {
			conn.Close()
		}
	}()
	for newConnection := range newConnectionChan {
		// A new connection has come in. If we already have a connection to this node, close the old one and replace it.
		conn, ok := ipToConnectionWrite.load(newConnection.Remote)
		if ok {
			logger.Debug(fmt.Sprint("Connection to ", newConnection.Remote, " already exists; overwriting it"))
			ipToConnectionWrite.store(newConnection.Remote, newConnection.Connection)
			conn.Close()
		} else {
			// Store the new connection
			logger.Debug(fmt.Sprint("Stored a new write connection to ", newConnection.Remote, ", ", newConnection.Connection.RemoteAddr()))
			ipToConnectionWrite.store(newConnection.Remote, newConnection.Connection)
		}
	}
}

func readConnectionManager(receiveChannel chan NetworkMessage, newConnectionChan chan net.Conn) {
	defer func() {
		for _, conn := range ipToConnectionRead.connMap {
			conn.Close()
		}
	}()
	for newConnection := range newConnectionChan {
		// A new connection has come in. If we already have a connection to this node, close the old one and replace it.
		conn, ok := ipToConnectionRead.load(newConnection.RemoteAddr().String())
		if ok {
			logger.Debug(fmt.Sprint("Received an already established connection from ", newConnection.RemoteAddr().String()))
			ipToConnectionWrite.store(newConnection.RemoteAddr().String(), newConnection)
			conn.Close()
		} else {
			// Store the new connection and start receiving
			go startReceiving(receiveChannel, newConnection)
			logger.Debug(fmt.Sprint("Stored a new connection from ", newConnection.RemoteAddr().String()))
			ipToConnectionRead.store(newConnection.RemoteAddr().String(), newConnection)
		}
	}
}

// Manages failed connections.
// This removes the connection from the connection map then returns the ID of the failed remote on the external channel.
// It also closes the connection to preserve resources.
func removeConnectionManager(failedSendsExternal chan string) {
	for namedConn := range removeWriteChan {
		logger.Warn(fmt.Sprint("Failed connection to ", namedConn.Remote, " registered, closing it..."))
		ipToConnectionWrite.remove(namedConn.Remote)
		namedConn.Connection.Close()
		failedSendsExternal <- namedConn.Remote
	}
}
