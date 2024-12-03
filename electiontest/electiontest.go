package main

import (
	"encoding/json"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/election"
	"local/zookeeper/internal/logger"
	"log/slog"
	"os"
	"time"
)

var config configReader.Config

func main() {
	mode := os.Getenv("MODE") // "Server" or "Client"
	handler := logger.NewPlainTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))
	if mode == "Server" {
		logger.Info("Server starting...")
		go client(os.Getenv("NAME"))
	}
	select {}
}

func client(address string) {
	recv, _ := connectionManager.Init()
	timeoutDuration := 10 * time.Second
	timeoutTimer := time.NewTimer(timeoutDuration)
	election.ElectionInit()
	if address == "server1" {

		election.InitiateElectionDiscovery()
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
			electionstatus := election.HandleMessage(messageWrapper)
			if electionstatus {
				logger.Info(fmt.Sprint("Election Ongoing"))
			} else {
				logger.Info(fmt.Sprint("Election Stop"))

			}

		case <-timeoutTimer.C:
			fmt.Println("Timeout occurred. Initiating election.")

			election.InitiateElectionDiscovery()
			timeoutTimer.Reset(timeoutDuration)
		}
	}
}
