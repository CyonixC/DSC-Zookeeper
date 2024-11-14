package main

import (
	"bufio"
	"fmt"
	"local/zookeeper/internal/znode"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func help() {
	fmt.Println("Available commands: create_session, delete_session, create, delete, set, exist, get, children, znode, watch, update_watch, help, exit")
}

func main() {
	err := znode.Init_znode_cache()
	if err != nil {
		fmt.Printf("Error initializing cache: %v\n", err)
		return
	}
	znode.Init_watch_cache()
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

			updated_req, err := znode.Check(req)
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

			updated_req, err := znode.Check(req)
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

			updated_req, err := znode.Check(req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			modified, err := znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error writing znode: %v\n", err)
			} else {
				fmt.Printf("Znode created with name: %s\n", filepath.Base(modified[0]))
			}

			reqs, watchers, err := znode.Check_watch(modified)
			if err != nil {
				fmt.Printf("Error checking watchers: %v\n", err)
				continue
			}
			fmt.Printf("Watchers: %v\n", watchers)
			for _, req := range reqs {
				//Skipping check step here, but by right should do
				_, err = znode.Write(req)
				if err != nil {
					fmt.Printf("Error propogating watch flag: %v\n", err)
				} else {
					fmt.Printf("Watch flag propogated\n")
				}
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

			updated_req, err := znode.Check(req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			modified, err := znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error deleting znode: %v\n", err)
			} else {
				fmt.Printf("Znode deleted\n")
			}

			reqs, watchers, err := znode.Check_watch(modified)
			if err != nil {
				fmt.Printf("Error checking watchers: %v\n", err)
				continue
			}
			fmt.Printf("Watchers: %v\n", watchers)
			for _, req := range reqs {
				//Skipping check step here, but by right should do
				_, err = znode.Write(req)
				if err != nil {
					fmt.Printf("Error propogating watch flag: %v\n", err)
				} else {
					fmt.Printf("Watch flag propogated\n")
				}
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

			updated_req, err := znode.Check(req)
			if err != nil {
				fmt.Printf("Error checking request: %v\n", err)
				continue
			}

			modified, err := znode.Write(updated_req)
			if err != nil {
				fmt.Printf("Error updating znode: %v\n", err)
			} else {
				fmt.Printf("Znode updated\n")
			}

			reqs, watchers, err := znode.Check_watch(modified)
			if err != nil {
				fmt.Printf("Error checking watchers: %v\n", err)
				continue
			}
			fmt.Printf("Watchers: %v\n", watchers)
			for _, req := range reqs {
				//Skipping check step here, but by right should do
				_, err = znode.Write(req)
				if err != nil {
					fmt.Printf("Error propogating watch flag: %v\n", err)
				} else {
					fmt.Printf("Watch flag propogated\n")
				}
			}

		case "exist":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			fmt.Print("Watch? (y/n): ")
			scanner.Scan()
			watch := strings.TrimSpace(scanner.Text()) == "y"

			exists := znode.Exists(path)

			if watch {
				req, err := znode.Encode_watch(sessionid, path, true)
				if err != nil {
					fmt.Printf("Error encoding request: %v\n", err)
					continue
				}
				fmt.Println("Watch Cache updated")

				updated_req, err := znode.Check(req)
				if err != nil {
					fmt.Printf("Error checking request: %v\n", err)
					continue
				}

				_, err = znode.Write(updated_req)
				if err != nil {
					fmt.Printf("Error propogating watch flag: %v\n", err)
				} else {
					fmt.Printf("Watch flag propogated\n")
				}
			}

			if exists {
				fmt.Println("Znode exists.")
			} else {
				fmt.Println("Znode does not exist.")
			}

		case "get":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			fmt.Print("Watch? (y/n): ")
			scanner.Scan()
			watch := strings.TrimSpace(scanner.Text()) == "y"

			info, err := znode.GetData(path)
			if err != nil {
				fmt.Printf("Error getting znode: %v\n", err)
			} else {
				// only apply watch flag if znode exists
				if watch {
					req, err := znode.Encode_watch(sessionid, path, true)
					if err != nil {
						fmt.Printf("Error encoding request: %v\n", err)
						continue
					}
					fmt.Println("Watch Cache updated")

					updated_req, err := znode.Check(req)
					if err != nil {
						fmt.Printf("Error checking request: %v\n", err)
						continue
					}

					_, err = znode.Write(updated_req)
					if err != nil {
						fmt.Printf("Error propogating watch flag: %v\n", err)
					} else {
						fmt.Printf("Watch flag propogated\n")
					}
				}
				znode.PrintZnode(info)
			}

		case "children":
			fmt.Print("Enter path: ")
			scanner.Scan()
			path := strings.TrimSpace(scanner.Text())

			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			fmt.Print("Watch? (y/n): ")
			scanner.Scan()
			watch := strings.TrimSpace(scanner.Text()) == "y"

			children, err := znode.GetChildren(path)
			if err != nil {
				fmt.Printf("Error getting children: %v\n", err)
			} else {
				// only apply watch flag if znode exists
				if watch {
					req, err := znode.Encode_watch(sessionid, path, true)
					if err != nil {
						fmt.Printf("Error encoding request: %v\n", err)
						continue
					}
					fmt.Println("Watch Cache updated")

					updated_req, err := znode.Check(req)
					if err != nil {
						fmt.Printf("Error checking request: %v\n", err)
						continue
					}

					_, err = znode.Write(updated_req)
					if err != nil {
						fmt.Printf("Error propogating watch flag: %v\n", err)
					} else {
						fmt.Printf("Watch flag propogated\n")
					}
				}
				fmt.Printf("Children: %v\n", children)
			}

		case "znode":
			fmt.Println("znode Cache contents:")
			znode.Print_znode_cache()

		case "watch":
			fmt.Println("watch Cache contents:")
			znode.Print_watch_cache()

		case "update_watch":
			fmt.Print("Enter session id: ")
			scanner.Scan()
			sessionid := strings.TrimSpace(scanner.Text())

			err := znode.Update_watch_cache(sessionid)
			if err != nil {
				fmt.Printf("Error updating watch cache: %v\n", err)
			} else {
				fmt.Println("Watch cache updated")
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
