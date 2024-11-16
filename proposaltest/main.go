package main

import (
	"fmt"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	prp "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/logger"
	"log/slog"
	"os"
	"time"
)

func identity(m []byte) ([]byte, error)  { return m, nil }
func rejectAll(m []byte) ([]byte, error) { return m, fmt.Errorf("denied") }

func main() {
	role := os.Getenv("MODE")
	handler := logger.NewPlainTextHandler(slog.LevelDebug)
	lg := slog.New(handler)
	logger.InitLogger(lg)
	recv_channel, failed := connectionManager.Init()
	commitChan, denied := prp.Init(recv_channel, rejectAll)
	go func() {
		for str := range failed {
			logger.Info("Failed to send to", str)
		}
	}()
	go func() {
		for deniedReq := range denied {
			logger.Warn(fmt.Sprint("Request #", deniedReq.ReqNumber, " denied"))
		}
	}()

	go func() {
		for msg := range commitChan {
			logger.Info(fmt.Sprint("Received message: ", msg))
		}
	}()
	if role == "Client" {
		var input string
		var requestCounter = 0
		for {
			fmt.Scanln(&input)
			logger.Info(fmt.Sprint("Sending message: ", input))
			prp.SendWriteRequest([]byte(input), requestCounter)
			requestCounter++
		}
	}
	time.Sleep(time.Hour)
}
