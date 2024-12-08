package main

// Fault tolerance tests for ZAB servers

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

// Return pass or fail with a 50-50 chance.
func randomPassFail(proposed []byte) ([]byte, error) {
	if rand.IntN(2) == 0 {
		return proposed, errors.New("Failed")
	} else {
		return proposed, nil
	}
}

func main() {
	// Initialiase logger
	handler := logger.NewColouredTextHandler(slog.LevelDebug)
	logger.InitLogger(slog.New(handler))

	// Initialise connection, proposal, and election layers
	recv, failed := connectionManager.Init()

	// Message processing loop
	go func() {
		// The elections run first. Pause the Proposals layer from sending anything while elections happen.
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
					// If an election is currently happening, stop ZAB from sending any messages.
					proposals.Pause()
				} else {
					// Otherwise, allow it to continue.
					proposals.Continue()
				}
			}
		}
	}()

	// Initialise with the random check function
	success, rejected, counter := proposals.Init(randomPassFail)
	election.ElectionInit(counter)
	if configReader.GetName() == "server1" {
		election.InitiateElectionDiscovery()
	}

	// Failed node processing loop
	go func() {
		for f := range failed {
			logger.Error(fmt.Sprint("Failed to send to ", f))
			if f == election.Coordinator.GetCoordinator() {
				proposals.Pause()
				go election.InitiateElectionDiscovery()
			}
		}
	}()

	// Approved COMMIT processing loop
	go func() {
		for s := range success {
			logger.Info(fmt.Sprint("Succeeded: ", string(s.Content)))
		}
	}()

	// Rejected request processing loop
	go func() {
		for r := range rejected {
			logger.Info(fmt.Sprint("Rejected request number ", r.ReqNumber))
		}
	}()

	// Send a new write request periodically
	for i := 0; ; i++ {
		time.Sleep(time.Second * 2)
		proposals.SendWriteRequest([]byte(fmt.Sprint("Test", i)), i)
	}
}
