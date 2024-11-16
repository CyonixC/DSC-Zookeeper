package main

import (
	"fmt"
	cxn "local/zookeeper/internal/ConnectionManager"
	prp "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/logger"
	"log"
	"log/slog"
	"os"
	"time"
)

func receiveMsg(recv_channel chan cxn.NetworkMessage) {
	for msg := range recv_channel {
		prp.ProcessZabMessage(msg)
	}
}

func identity(m []byte) ([]byte, error) { return m, nil }

func main() {
	role := os.Getenv("MODE")
	handler := logger.NewPlainTextHandler(slog.LevelDebug)
	lg := slog.New(handler)
	logger.InitLogger(lg)
	recv_channel, failed := cxn.Init()
	commitChan := prp.Init(identity)
	go receiveMsg(recv_channel)
	go func() {
		for str := range failed {
			log.Println("Failed to send to", str)
		}
	}()

	go func() {
		for msg := range commitChan {
			logger.Info(fmt.Sprint("Received message: ", msg))
		}
	}()
	if role == "Client" {
		var input string
		for {
			fmt.Scan(&input)
			prp.SendWriteRequest([]byte(input))
		}
	}
	time.Sleep(time.Hour)
}
