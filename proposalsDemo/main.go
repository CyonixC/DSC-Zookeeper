package main

import (
	"fmt"
	cxn "local/zookeeper/internal/LocalConnectionManager"
	prp "local/zookeeper/internal/Proposals"
	"log"
	"math/rand/v2"
	"net"
	"runtime"
	"time"
)

var addresses = []string{"192.168.10.1", "192.168.10.2", "192.168.10.3", "192.168.10.4", "192.168.10.5"}

func client(selfIP net.Addr) {
	recv_channel := cxn.Init(selfIP)
	failedSends := make(chan string)
	go receiveMsg(recv_channel, failedSends, selfIP)
	go func() {
		for str := range failedSends {
			log.Println(selfIP, "failed to send to", str)
		}
	}()
	for {
		time.Sleep(time.Second)
		if selfIP.String() != "192.168.10.1" {
			continue
		}
		newVar := []byte{byte(rand.IntN(256))}
		fmt.Println("Request sending:", newVar)
		prp.SendWriteRequest(newVar, failedSends, selfIP)
		fmt.Println("Request sent")
		fmt.Println("Number of goroutines:", runtime.NumGoroutine())
	}
}

func receiveMsg(recv_channel chan cxn.NetworkMessage, failedSends chan string, selfIP net.Addr) {
	for msg := range recv_channel {
		go prp.ProcessZabMessage(msg, failedSends, selfIP)
	}
}

func main() {
	for _, addr := range addresses {
		netaddr, _ := net.ResolveIPAddr("ip", addr)
		go client(netaddr)
	}
	var input string
	fmt.Scanln(&input)
}
