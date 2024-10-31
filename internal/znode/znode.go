package znode

import (
	"os"
	"path/filepath"
	"strconv"
)

// currently store znodes in this project's directory
// path structure: ./znodeDir/znodePath/versionNumber.txt
// meaning working directory for server must be directory containing znodeDir (prob root of project)
// version number can just be an integer, incremented on each write
// this allows znodes to have children (and may help with fault tolerance later?)
// .txt used for ease of reading and debugging
const znodeDir = "znodeDir"

type ZNode struct {
	//path provided by client will be a relative path in znodeDir, corresponding to znodepath above
	path    string //TODO func to modify filepath to match local sys and include seq number
	data    []byte
	version int
	//TODO flags and stuff
}

// writeZNode creates a znode in the filesystem.
// does not check if all dir(znode) in path exists, will error if they do not
func writeZNode(znode *ZNode) {
	path := filepath.Join(".", znodeDir, znode.path, strconv.Itoa(znode.version)+".txt")

	//create a file in the path
	err2 := os.WriteFile(path, znode.data, 0644)
	if err2 != nil {
		panic(err2)
	}
}

// readZnode reads a znode from the filesystem.
// Updates the data field of the znode.
// Does not handle version checking.
func readZnode(znode *ZNode) {
	path := filepath.Join(".", znodeDir, znode.path, strconv.Itoa(znode.version)+".txt")
	//read the file in the path
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	znode.data = data
}

// deleteZnode deletes a znode (version) from the filesystem.
// Assumes the znode exists.
// Does not handle version checking.
func deleteZnode(znode *ZNode) {
	path := filepath.Join(".", znodeDir, znode.path, strconv.Itoa(znode.version)+".txt")
	err := os.Remove(path)
	if err != nil {
		panic(err)
	}
}
