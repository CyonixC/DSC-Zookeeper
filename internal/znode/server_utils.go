package znode

//utils for znode server process, generally functions to be called upon receiving corresponding API requests from clients
//generally avoid interacting with file system directly, use functions from znode.go instead
//Meant to focus on server-side logic i.e. version control, watch flag, etc.
//TODO stuff to sync with other servers

import (
	"os"
	"path/filepath"
)

// InitZnodeServer ensures the znode directory exists and returns an empty znode cache for znode instances.
func InitZnodeServer() map[string]*ZNode {
	//create a directory for znodes, mkdirall will not error if it already exists
	err := os.MkdirAll(filepath.Join(".", znodeDir), 0755)
	if err != nil {
		panic(err)
	}
	// Cache is meant to reduce unnecessary interactions with file system.
	// znode might be in storage but not in cache. Vice versa should not happen.
	// Might be safer to not use cache and always read from storage, not sure.
	znodeCache := make(map[string]*ZNode)
	return znodeCache
}

func Create(path string, data []byte, ephemeral bool, sequential bool) {

}

func Delete(path string, version int) {

}

func Exists(path string, watch bool) {

}

func GetData(path string, watch bool) {

}

func SetData(path string, data []byte, version int) {

}

func GetChildren(path string, watch bool) {

}

func Sync(path string) {

}
