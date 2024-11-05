package main

import (
	"bufio"
	"fmt"
	"local/zookeeper/internal/znode"
	"os"
	"strconv"
	"strings"
)

func help() {
	fmt.Println("Available commands: create, delete, set, exist, get, children, help, exit")
}

func main() {
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

			check, err := znode.Check(req)
			if !check {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			name, err := znode.Write(req)
			if err != nil {
				fmt.Printf("Error creating znode: %v\n", err)
			} else {
				fmt.Printf("Znode created: %s\n", name)
			}

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

			check, err := znode.Check(req)
			if !check {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			_, err = znode.Write(req)
			if err != nil {
				fmt.Printf("Error deleting znode: %v\n", err)
			} else {
				fmt.Printf("Znode deleted: %s\n", path)
			}

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

			check, err := znode.Check(req)
			if !check {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			_, err = znode.Write(req)
			if err != nil {
				fmt.Printf("Error setting data: %v\n", err)
			} else {
				fmt.Printf("Znode updated: %s\n", path)
			}

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
