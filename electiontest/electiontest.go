package main

import (
	"encoding/json"
	"fmt"
	"local/zookeeper/election"
	connectionManager "local/zookeeper/internal/ConnectionManager"
	"local/zookeeper/internal/logger"
	"time"
)


type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}
var addresses = []string{"server1", "server2", "sever3"}
func main() {
	for i := 0; i < len(addresses); i++ {
		fmt.Print("Start Client\n", addresses[i])
		go client(addresses[i])
	}

	select{}
}
func client(address string) {
	ringStruct, failedChan := election.ElectionInit(addresses, address)
	recv, _ := connectionManager.Init()

	timeoutDuration := 1 * time.Second
	timeoutTimer := time.NewTimer(timeoutDuration)

	for {
		select {
		case receivedMsg := <-recv:
			fmt.Println("Received Message %s", address)
			timeoutTimer.Reset(timeoutDuration)

			var messageWrapper election.MessageWrapper
			err := json.Unmarshal(receivedMsg.Message, &messageWrapper)
			if err != nil {
				logger.Fatal(fmt.Sprint("Error unmarshalling message:", err))
			}
			election.HandleMessage(address, ringStruct, failedChan, messageWrapper)

		case <-timeoutTimer.C: // Timeout occurred
			fmt.Println("Timeout occurred. Initiating election.")
			election.InitiateElectionDiscovery(address, ringStruct, failedChan)

			timeoutTimer.Reset(timeoutDuration)
		}
	}
}
