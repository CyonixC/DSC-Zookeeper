package main

import (
	"bufio"
	"fmt"
	cxn "local/zookeeper/internal/LocalConnectionManager"
	prp "local/zookeeper/internal/Proposals"
	"local/zookeeper/internal/znode"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

var addresses = []string{"192.168.10.1", "192.168.10.2", "192.168.10.3", "192.168.10.4", "192.168.10.5"}

func help() {
	fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
}

func client(selfIP net.Addr) {
	recv_channel := cxn.Init(selfIP)
	failedSends := make(chan string)
	go receiveMsg(recv_channel, failedSends, selfIP)
	go func() {
		for str := range failedSends {
			log.Println(selfIP, "failed to send to", str)
		}
	}()
	if selfIP.String() == "192.168.10.6" {
		interactive(failedSends, selfIP)
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
	netaddr, _ := net.ResolveIPAddr("ip", "192.168.10.6")
	client(netaddr)
}

func interactive(failedSends chan string, ip net.Addr) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Interactive Mode. Type 'help' for a list of commands.")
	help()

	for {
		fmt.Print("> ")
		scanner.Scan() // Reads the input
		command := strings.TrimSpace(scanner.Text())

		switch command {

		case "create":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter data: ")
			scanner.Scan()
			input := strings.TrimSpace(scanner.Text())
			data := []byte(input)

			fmt.Print("Sequential? (y/n): ")
			scanner.Scan()
			sequential := strings.TrimSpace(scanner.Text()) == "y"

			req, err := znode.Encode_write_request("create", path, data, 0, false, sequential)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			prp.SendWriteRequest(req, failedSends, ip)

		case "delete":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter version: ")
			scanner.Scan()
			version, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			if err != nil {
				fmt.Printf("invalid version: %v\n", err)
				continue
			}

			req, err := znode.Encode_write_request("delete", path, nil, version, false, false)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			prp.SendWriteRequest(req, failedSends, ip)

		case "set":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter data: ")
			scanner.Scan()
			input := strings.TrimSpace(scanner.Text())
			data := []byte(input)

			fmt.Print("Enter version: ")
			scanner.Scan()
			version, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			if err != nil {
				fmt.Printf("invalid version: %v\n", err)
				continue
			}

			req, err := znode.Encode_write_request("setdata", path, data, version, false, false)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			prp.SendWriteRequest(req, failedSends, ip)

		case "exist":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			exists := znode.Exists(path)

			if exists {
				fmt.Println("Znode exists.")
			} else {
				fmt.Println("Znode does not exist.")
			}

		case "get":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			znode, err := znode.GetData(path)
			if err != nil {
				fmt.Printf("Error getting znode: %v\n", err)
			} else {
				fmt.Printf("Data: %s\n", znode.Data)
				fmt.Printf("Version: %d\n", znode.Version)
			}

		case "children":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			children, err := znode.GetChildren(path)
			if err != nil {
				fmt.Printf("Error getting children: %v\n", err)
			} else {
				fmt.Printf("Children: %v\n", children)
			}

		case "help":
			help()

		case "exit":
			fmt.Println("Exiting program.")
			return

		default:
			fmt.Printf("Unknown command: %s\n", command)
			help()
		}
	}
}
