package znode

//utils for znode server process, generally functions to be called upon receiving corresponding API requests from clients
//generally avoid interacting with file system directly, use functions from znode.go instead
//Meant to focus on server-side logic i.e. version control, watch flag, etc.
//TODO stuff to sync with other servers

import (
	"errors"
	"os"
	"path/filepath"
)

// InitZnodeServer ensures the znode directory exists and returns an empty znode cache for znode instances.
func InitZnodeServer() (map[string]*ZNode, error) {
	//create a directory for znodes, mkdirall will not error if it already exists
	err := os.MkdirAll(filepath.Join(".", znodeDir), 0755)
	if err != nil {
		return nil, err
	}
	// Cache is meant to reduce unnecessary interactions with file system.
	// znode might be in storage but not in cache. Vice versa should not happen.
	// Might be safer to not use cache and always read from storage, not sure.
	znodeCache := make(map[string]*ZNode)
	return znodeCache, nil
}

// Create creates a znode in the filesystem and adds to cache.
// Returns the name of the znode created, or an error if znode already exists
// If sequential flag is set, znode name will be modified to include a sequence number
func Create(znodeCache map[string]*ZNode, path string, data []byte, ephemeral bool, sequential bool) (string, error) {

	if sequential {
		//TODO modify path according to sequential flag behaviour, probably making us of childrenZnode(path)
	}

	//version numbering starting at 1
	_, err := createZnode(znodeCache, path, 1, ephemeral, data)

	if err != nil {
		return "", err
	}

	//get last part of path, i.e. the znode name
	name := filepath.Base(path)
	return name, nil

}

// Delete deletes a znode in the filesystem and removes from cache.
// currently if version matches, is a recursive delete, do we newed to check for children?
func Delete(znodeCache map[string]*ZNode, path string, version int) error {
	znode, err := getZnode(znodeCache, path)
	if err != nil {
		return err
	}

	if znode.Version != version {
		return errors.New("version does not match")
	}

	err = deleteZnode(znode)
	if err != nil {
		return err
	}
	//TODO watch flag behaviour
	//remove from cache
	znodeCache[path] = nil

	return nil
}

// Exists checks if a znode corresponding to the provided path exists.
func Exists(znodeCache map[string]*ZNode, path string, watch bool) bool {
	exists := existsZnode(znodeCache, path)

	if exists && watch {
		//TODO watch flag behaviour
	}

	return exists

}

// GetData retrieves the data of a znode from the filesystem.
// Also returns the znode struct for relavant metadata.
func GetData(znodeCache map[string]*ZNode, path string, watch bool) ([]byte, *ZNode, error) {
	znode, err := getZnode(znodeCache, path)
	if err != nil {
		return nil, nil, err
	}
	data, err := readZnode(znode)
	if watch {
		//TODO watch flag behaviour
	}
	//assumes relavant metadata stored in znode struct, i.e. version
	return data, znode, err

}

func SetData(znodeCache map[string]*ZNode, path string, data []byte, version int) error {
	znode, err := getZnode(znodeCache, path)
	if err != nil {
		return err
	}

	if znode.Version != version {
		return errors.New("version does not match")
	}

	//increment version number and write
	//currently just leaves old version in storage, may be useful for fault tolerance?
	znode.Version++
	err = writeZNode(znode, data)
	if err != nil {
		return err
	}

	//TODO watch flag behaviour

	return nil
}

// GetChildren retrieves the children of a znode from the filesystem.
func GetChildren(znodeCache map[string]*ZNode, path string, watch bool) ([]string, error) {
	exists := existsZnode(znodeCache, path)
	if !exists {
		return nil, errors.New("znode does not exist")
	}

	children, err := childrenZnode(path)
	if err != nil {
		return nil, err
	}

	if watch {
		//TODO watch flag behaviour
	}

	return children, nil

}

func Sync(path string) {

}
