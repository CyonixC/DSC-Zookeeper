package main

import (
	"bufio"
	"fmt"
	cxn "local/zookeeper/internal/LocalConnectionManager"
	"net"
	"os"
	"strings"
)

var addresses = []string{"192.168.10.0", "192.168.10.1", "192.168.10.2"}
var client_ip = "192.168.11.0"

func main() {
	client_netaddr, _ := net.ResolveIPAddr("ip", client_ip)
	client_recv_channel := cxn.Init(client_netaddr)
	client_failedSend := make(chan string)
	go listener(client_recv_channel, client_failedSend, client_netaddr)

	for i := 0; i < len(addresses); i++ {
		server_netaddr, _ := net.ResolveIPAddr("ip", addresses[i])
		message := cxn.NetworkMessage{Remote: server_netaddr, Message: []byte("Hello")}
		go cxn.SendMessage(message, client_netaddr, client_failedSend)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		scanner.Scan()
		command := strings.TrimSpace(scanner.Text())

		switch command {

		case "exit":
			fmt.Println("Exiting program.")
			return

		default:
			fmt.Printf("Unknown command: %s\n", command)
			help()
		}
	}
}

func help() {
	fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
}

// Listen for messages
func listener(recv_channel chan cxn.NetworkMessage, failedSend chan string, selfIP net.Addr) {
	for {
		select {
		case msg := <-failedSend:
			fmt.Printf("Failed to send message: %s\n> ", msg)
		case network_msg := <-recv_channel:
			fmt.Printf("Receive message from %s\n> ", network_msg.Remote)
		}
	}
}
