package main

import (
	"encoding/json"
	"errors"
	"fmt"
	configReader "local/zookeeper/internal/ConfigReader"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	proposals "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/election"
	"local/zookeeper/internal/logger"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Fault tolerance tests for ZAB servers
func randomPassFail(proposed []byte) ([]byte, error) {
	if rand.IntN(2) == 0 {
		return proposed, errors.New("Failed")
	} else {
		return proposed, nil
	}
}

func main() {
	handler := logger.NewColouredTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))
	time.Sleep(time.Second)
	recv, failed := connectionManager.Init()
	time.Sleep(time.Second)
	go func() { //Removed listener from proposals package, call ProcessZabMessage manually
		electing := false
		for network_msg := range recv {
			switch network_msg.Type {
			case connectionManager.ZAB:
				if electing {
					// If election currently happening, enqueue all ZAB messages
					proposals.StoreZabMessage(network_msg)
				} else {
					proposals.ProcessZabMessage(network_msg)
				}
			case connectionManager.ELECTION:
				var messageWrapper election.MessageWrapper
				err := json.Unmarshal(network_msg.Message, &messageWrapper)
				if err != nil {
					logger.Fatal(fmt.Sprint("Error unmarshalling message:", err))
				}
				currentlyElecting := election.HandleMessage(messageWrapper)
				if electing && !currentlyElecting {
					// If the election has just finished, process all Zab messages received during the election.
					proposals.EmptyMessageQueue()
				}
				electing = currentlyElecting
			}
		}
	}()
	success, rejected, counter := proposals.Init(randomPassFail)
	if counter == nil {
		logger.Fatal("Failed")
	}
	election.ElectionInit(counter)
	time.Sleep(10 * time.Second)
	if configReader.GetName() == "server1" {
		election.InitiateElectionDiscovery()
	}
	time.Sleep(10 * time.Second)

	go func() {
		for f := range failed {
			logger.Error(fmt.Sprint("Failed to send to ", f))
			if f == election.Coordinator.GetCoordinator() {
				go election.InitiateElectionDiscovery()
			}
		}
	}()

	go func() {
		for s := range success {
			logger.Info(fmt.Sprint("Succeeded: ", string(s.Content)))
		}
	}()

	go func() {
		for r := range rejected {
			logger.Info(fmt.Sprint("Rejected request number ", r.ReqNumber))
		}
	}()

	for i := 0; ; i++ {
		time.Sleep(time.Second)
		proposals.SendWriteRequest([]byte(fmt.Sprint("Test", i)), i)
	}
}
