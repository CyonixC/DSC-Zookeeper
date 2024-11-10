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
	fmt.Println("Available commands: create_session, delete_session, create, delete, set, exist, get, children, cache, help, exit")
}

func main() {
	cache, err := znode.Init_cache()
	if err != nil {
		fmt.Printf("Error initializing cache: %v\n", err)
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Interactive Mode. Type 'help' for a list of commands.")
	help()

	for {
		fmt.Println()
		fmt.Print("> ")
		scanner.Scan() // Reads the input
		command := strings.TrimSpace(scanner.Text())

		switch command {

		case "create_session":
			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			req, err := znode.Encode_create_session(sessionid)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			updated_req, err := znode.Check(cache, req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			_, err = znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error creating session: %v\n", err)
			} else {
				fmt.Printf("Session created with id: %s\n", sessionid)
			}

		case "delete_session":
			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			req, err := znode.Encode_delete_session(sessionid)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			updated_req, err := znode.Check(cache, req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			_, err = znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error deleting session: %v\n", err)
			} else {
				fmt.Printf("Session deleted with id: %s\n", sessionid)
			}
		case "create":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter data: ")
			scanner.Scan()
			input := strings.TrimSpace(scanner.Text())
			data := []byte(input)

			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			fmt.Print("Sequential? (y/n): ")
			scanner.Scan()
			sequential := strings.TrimSpace(scanner.Text()) == "y"

			fmt.Print("Ephemeral? (y/n): ")
			scanner.Scan()
			ephemeral := strings.TrimSpace(scanner.Text()) == "y"

			req, err := znode.Encode_create(path, data, ephemeral, sequential, sessionid)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			updated_req, err := znode.Check(cache, req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			name, err := znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error writing znode: %v\n", err)
			} else {
				fmt.Printf("Znode created with name: %s\n", name)
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

			req, err := znode.Encode_delete(path, version)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			updated_req, err := znode.Check(cache, req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			_, err = znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error deleting znode: %v\n", err)
			} else {
				fmt.Printf("Znode deleted\n")
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

			req, err := znode.Encode_setdata(path, data, version)
			if err != nil {
				fmt.Printf("Error encoding request: %v\n", err)
				continue
			}

			updated_req, err := znode.Check(cache, req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			_, err = znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error updating znode: %v\n", err)
			} else {
				fmt.Printf("Znode updated\n")
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

			info, err := znode.GetData(path)
			if err != nil {
				fmt.Printf("Error getting znode: %v\n", err)
			} else {
				znode.PrintZnode(info)
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

		case "cache":
			fmt.Println("Cache contents:")
			znode.Print_cache(cache)

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
