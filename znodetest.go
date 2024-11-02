package main

import (
	"local/zookeeper/internal/znode"
)

func main() {
	cache := znode.InitZnodeServer()
	//lol created this to test znode utils, but realised I don't want to export those, so just leaving this as example of importing znode pkg
	//TODO: implement server_utils and test
}
