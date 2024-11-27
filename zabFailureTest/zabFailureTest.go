package main

import (
	"errors"
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	proposals "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/logger"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Fault tolerance tests for ZAB servers
func randomPassFail(proposed []byte) ([]byte, error) {
	if rand.IntN(2) == 0 {
		logger.Debug(fmt.Sprint("Current ZXID: ", proposals.ZxidCounter.GetLatestZXID()))
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
	success, rejected := proposals.Init(recv, randomPassFail)
	time.Sleep(time.Second)

	go func() {
		for f := range failed {
			logger.Fatal(fmt.Sprint("Failed to send to ", f))
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
