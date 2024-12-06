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
	recv, failed := connectionManager.Init()
	go func() { //Removed listener from proposals package, call ProcessZabMessage manually
		proposals.Pause()
		for network_msg := range recv {
			switch network_msg.Type {
			case connectionManager.ZAB:
				proposals.EnqueueZabMessage(network_msg)
			case connectionManager.ELECTION:
				var messageWrapper election.MessageWrapper
				err := json.Unmarshal(network_msg.Message, &messageWrapper)
				if err != nil {
					logger.Fatal(fmt.Sprint("Error unmarshalling message:", err))
				}
				currentlyElecting := election.HandleMessage(messageWrapper)
				if currentlyElecting {
					proposals.Pause()
				} else {
					proposals.Continue()
				}
			}
		}
	}()
	success, rejected, counter := proposals.Init(randomPassFail)
	election.ElectionInit(counter)
	if configReader.GetName() == "server1" {
		election.InitiateElectionDiscovery()
	}

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
