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
	fmt.Println("Available commands: create, delete, exist, get, set, children, help, exit")
}

func main() {
	cache, _ := znode.InitZnodeServer()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Interactive Mode. Type 'help' for a list of commands.")

	for {
		help()
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

			fmt.Print("Ephemeral? (y/n): ")
			scanner.Scan()
			ephemeral := strings.TrimSpace(scanner.Text()) == "y"

			fmt.Print("Sequential? (y/n): ")
			scanner.Scan()
			sequential := strings.TrimSpace(scanner.Text()) == "y"

			name, err := znode.Create(cache, path, data, ephemeral, sequential)
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

			err = znode.Delete(cache, path, version)
			if err != nil {
				fmt.Printf("Error deleting znode: %v\n", err)
			} else {
				fmt.Println("Znode deleted.")
			}

		case "exist":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Watch? (y/n): ")
			scanner.Scan()
			watch := strings.TrimSpace(scanner.Text()) == "y"

			exists := znode.Exists(cache, path, watch)

			if exists {
				fmt.Println("Znode exists.")
			} else {
				fmt.Println("Znode does not exist.")
			}

		case "get":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Watch? (y/n): ")
			scanner.Scan()
			watch := strings.TrimSpace(scanner.Text()) == "y"

			data, znode, err := znode.GetData(cache, path, watch)
			if err != nil {
				fmt.Printf("Error getting znode: %v\n", err)
			} else {
				fmt.Printf("Data: %s\n", data)
				fmt.Printf("Version: %d\n", znode.Version)
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

			err = znode.SetData(cache, path, data, version)
			if err != nil {
				fmt.Printf("Error setting znode: %v\n", err)
			} else {
				fmt.Println("Znode set.")
			}

		case "children":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Watch? (y/n): ")
			scanner.Scan()
			watch := strings.TrimSpace(scanner.Text()) == "y"

			children, err := znode.GetChildren(cache, path, watch)
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
