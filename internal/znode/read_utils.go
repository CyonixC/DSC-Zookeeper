package znode

// functions in this file correspond to the client API read operations of the same name

import (
	"os"
	"path/filepath"
)

// Exists checks if a znode exists.
func Exists(path string) bool {
	return existsZnode(path)
}

// GetData returns a znode with the specified path.
// gets latest version of the znode as well as data.
// TODO no ephemeral/sequential data for now.
func GetData(path string) (*ZNode, error) {
	if existsZnode(path) {
		version, err := latestVersion(path)
		if err != nil {
			return nil, err
		}
		znode := &ZNode{
			Path:    path,
			Data:    nil,
			Version: version,
		}
		//read the znode data
		err = readZnode(znode)
		if err != nil {
			return nil, err
		}

		return znode, nil
	}

	//return nil if znode does not exist
	return nil, &ExistsError{"znode does not exist"}
}

// children returns the children of a znode.
// Will be empty if znode has no children.
func GetChildren(path string) ([]string, error) {

	if existsZnode(path) {
		//get all files in the path
		entries, err := os.ReadDir(filepath.Join(".", znodeDir, path))
		if err != nil {
			return nil, err
		}

		children := make([]string, 0)
		for _, entry := range entries {
			//only add directories(children znodes)
			if entry.IsDir() {
				children = append(children, entry.Name())
			}
		}
		return children, nil
	}
	return nil, &ExistsError{"znode does not exist"}
}
