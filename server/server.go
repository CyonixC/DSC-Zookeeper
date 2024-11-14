package main

import (
	"bufio"
	"fmt"
	cxn "local/zookeeper/internal/LocalConnectionManager"
	"net"
	"os"
	"strconv"
	"strings"
)

var addresses = []string{"192.168.10.0", "192.168.10.1", "192.168.10.2"}

func main() {
	// Access individual arguments (excluding the program name)
	if len(os.Args) != 2 {
		fmt.Printf("Please specify IP address 1 to %d\n", len(addresses))
		return
	}
	arg := os.Args[1]
	ip_arg, err := strconv.Atoi(arg)
	if err != nil || ip_arg > len(addresses) {
		fmt.Printf("Please specify IP address 1 to %d\n", len(addresses))
		return
	}

	server_netaddr, _ := net.ResolveIPAddr("ip", addresses[ip_arg])
	server_recv_channel := cxn.Init(server_netaddr)
	server_failedSend := make(chan string)
	go listener(server_recv_channel, server_failedSend, server_netaddr)

	fmt.Printf("Server started on %s\n", addresses[ip_arg])

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		scanner.Scan() // Reads the input
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
