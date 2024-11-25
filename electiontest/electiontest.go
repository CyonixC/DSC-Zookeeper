package main

import (
	"encoding/json"
	"fmt"
	"local/zookeeper/election"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"log/slog"
	"os"
	"time"
)
var config configReader.Config
var addresses = []string{"server1", "server2", "server3", "server4"}
var testbool = 1
func main() {
	mode := os.Getenv("MODE") // "Server" or "Client"
	handler := logger.NewPlainTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))
	if mode == "Server" {
		logger.Info("Server starting...")
		go client(os.Getenv("NAME"))
	}
	select{}
}

func client(address string) {
	recv, failedchan := connectionManager.Init()

	timeoutDuration := 10 * time.Second
	timeoutTimer := time.NewTimer(timeoutDuration)
	if address=="server4" && testbool==1{
		testbool=0
		election.InitiateElectionDiscovery(address, failedchan)
	}
	for {
		select {
		case receivedMsg := <-recv:
			fmt.Printf("Received Message %s from %s\n", receivedMsg, address)
			timeoutTimer.Reset(timeoutDuration)
			var messageWrapper election.MessageWrapper
			err := json.Unmarshal(receivedMsg.Message, &messageWrapper)
			if err != nil {
				logger.Fatal(fmt.Sprint("Error unmarshalling message:", err))
			}
			election.HandleMessage(address, failedchan, messageWrapper)
		case <-timeoutTimer.C: 
			fmt.Println("Timeout occurred. Initiating election.")
			election.InitiateElectionDiscovery(address, failedchan)
			timeoutTimer.Reset(timeoutDuration)
		}
	}
}
