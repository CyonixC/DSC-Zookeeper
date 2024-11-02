package znode

//functions in this file are not meant to be exported, only used by other znode package files

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
// also ensures children can only be created if parent znode exists
// .txt used for ease of reading and debugging
const znodeDir = "znodeDir"

type ZNode struct {
	//path provided by client will be a relative path in znodeDir, corresponding to znodepath above
	path string //TODO func to modify filepath to match local sys and include seq number, to be handled by server_utils prob using childrenZnode
	//Not sure if path should be stored in znode, might be unnecessary with cache, but leaving for now
	// data       []byte //tentatively don't want to store data here, should not need it since only related ops are read and write
	version    int
	ephermeral bool
}

// createZnode creates a znode in the filesystem and returns the znode.
// will panic if znode already exists
func createZnode(znodeCache map[string]*ZNode, path string, version int, ephermeral bool, data []byte) *ZNode {
	if existsZnode(znodeCache, path) {
		panic("Znode already exists")
	}

	znode := &ZNode{
		path:       path,
		version:    version, //May not start at 1, i.e. syncing with other servers
		ephermeral: ephermeral,
	}
	znodeCache[path] = znode
	writeZNode(znode, data)

	return znode
}

// writeZNode creates a znode in the filesystem.
// does not check if all dir(znode parents) in path exists, will error if they do not
func writeZNode(znode *ZNode, data []byte) {
	path := filepath.Join(".", znodeDir, znode.path, strconv.Itoa(znode.version)+".txt")

	//create a file in the path
	err2 := os.WriteFile(path, data, 0644)
	if err2 != nil {
		panic(err2)
	}
}

// readZnode reads a znode from the filesystem.
// Updates the data field of the znode.
// Does not handle version checking.
func readZnode(znode *ZNode) []byte {
	path := filepath.Join(".", znodeDir, znode.path, strconv.Itoa(znode.version)+".txt")
	//read the file in the path
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return data
}

// deleteZnode deletes a znode (version) from the filesystem.
// Assumes the znode exists.
// Does not handle version checking.
func deleteZnode(znode *ZNode) bool {
	path := filepath.Join(".", znodeDir, znode.path, strconv.Itoa(znode.version)+".txt")
	err := os.Remove(path)
	if err != nil {
		panic(err)
	}
	//tentatively returns true but prob unnecessary
	return true
}

// existsZnode checks if a znode with specified path exists.
func existsZnode(znodeCache map[string]*ZNode, path string) bool {
	//check if znode exists in cache
	if _, exists := znodeCache[path]; exists {
		return true
	}

	info, err := os.Stat(filepath.Join(".", znodeDir, path))
	if os.IsNotExist(err) || !info.IsDir() {
		//return false of path does not exist/does not point to a dir
		return false
	}
	return true
}

// latestVersion returns the latest stored version of a znode.
// Not sure if need, might be useful
func latestVersion(path string) int {
	//get all files in the path
	entries, err := os.ReadDir(filepath.Join(".", znodeDir, path))
	if err != nil {
		panic(err)
	}

	//find the latest version
	latest := 0
	for _, entry := range entries {
		//skip directories(children znodes)
		if entry.IsDir() {
			continue
		}

		//parse the filename to get the version number
		version, err := strconv.Atoi(entry.Name()[:len(entry.Name())-4])
		//should error if the filename is not a number
		if err != nil {
			panic(err)
		}

		if version > latest {
			latest = version
		}
	}
	return latest
}

// getZnode returns a znode with the specified path.
// If not in cache will check if in storage
// not sure if need, meant for cases where znode is not in cache but is in storage
func getZnode(znodeCache map[string]*ZNode, path string) *ZNode {
	if znode, exists := znodeCache[path]; exists {
		return znode
	}
	if existsZnode(znodeCache, path) {
		znode := &ZNode{
			path:       path,
			version:    latestVersion(path),
			ephermeral: false, //This might be an issue, tentatively assuming if znode instance is not in cache but in storage, it is not ephermeral
		}
		znodeCache[path] = znode
		return znode
	}
	//return nil if znode does not exist
	return nil
}

// children returns the children of a znode.
// Will be empty if znode has no children.
func childrenZnode(path string) []string {
	//get all files in the path
	entries, err := os.ReadDir(filepath.Join(".", znodeDir, path))
	if err != nil {
		panic(err)
	}

	children := make([]string, 0)
	for _, entry := range entries {
		//only add directories(children znodes)
		if entry.IsDir() {
			children = append(children, entry.Name())
		}
	}
	return children
}
