package znode

//functions in this file are not meant to be exported, only used by other znode package files

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	//Path provided by client will be a relative Path in znodeDir, corresponding to znodepath above
	Path string //TODO func to modify filepath to match local sys and include seq number, to be handled by server_utils prob using childrenZnode
	//Not sure if path should be stored in znode, might be unnecessary with cache, but leaving for now
	// data       []byte //tentatively don't want to store data here, should not need it since only related ops are read and write
	Version    int
	Ephermeral bool
	//TODO implement watch, this will have to be session specific? Not sure yet
}

// createZnode creates a znode in the filesystem and adds to cache.
// will error if znode already exists, check and handle
func createZnode(znodeCache map[string]*ZNode, path string, version int, ephermeral bool, data []byte) (*ZNode, error) {
	if existsZnode(znodeCache, path) {
		return nil, errors.New("znode already exists")
	}

	//check if parent znode exists, if not, znode cannot be created
	parentpath := filepath.Dir(path)
	if parentpath != "." && !existsZnode(znodeCache, parentpath) {
		return nil, errors.New("parent znode does not exist")
	}
	//check if parent znode is ephermeral, if so, znode cannot be created
	//Assumes that if ephemeral, znode will be in cache, ephemeral info currently not stored in filesystem
	//Might want to move parent and ephermeral check to server side
	parentznode, err := getZnode(znodeCache, parentpath)
	if err != nil {
		return nil, err
	}
	if parentznode.Ephermeral {
		return nil, errors.New("parent znode is ephermeral, it cannot have children")
	}

	znode := &ZNode{
		Path:       path,
		Version:    version, //May not start at 1, i.e. syncing with other servers
		Ephermeral: ephermeral,
	}
	err = writeZNode(znode, data)
	if err != nil {
		return nil, err
	}

	znodeCache[path] = znode

	return znode, nil
}

// writeZNode creates/overwrites znode in the filesystem, returns nil if successful.
// Does not handle version checking and will overwrite if version exists, be sure to check version
func writeZNode(znode *ZNode, data []byte) error {
	path := filepath.Join(".", znodeDir, znode.Path)
	//create dir for znode
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	//create a file in the path
	//TODO modify implementation such that will error if overwriting existing version (if we establish that should not happen)
	path = filepath.Join(path, strconv.Itoa(znode.Version)+".txt")
	err = os.WriteFile(path, data, 0644)
	return err
}

// readZnode reads a znode of from the filesystem.
// Does not handle version checking.
func readZnode(znode *ZNode) ([]byte, error) {
	path := filepath.Join(".", znodeDir, znode.Path, strconv.Itoa(znode.Version)+".txt")
	//read the file in the path
	data, err := os.ReadFile(path)
	return data, err
}

// deleteZnode deletes a znode (version) from the filesystem, returns nil if successful.
// Does not handle version checking.
func deleteZnodeVer(znode *ZNode) error {
	path := filepath.Join(".", znodeDir, znode.Path, strconv.Itoa(znode.Version)+".txt")
	err := os.Remove(path)
	return err
}

// deleteZnode deletes a znode from the filesystem, returns nil if successful.
// Does not handle version checking nor check for children znodes.
// Remember to remove from cache after calling this function.
func deleteZnode(znode *ZNode) error {
	err := os.RemoveAll(filepath.Join(".", znodeDir, znode.Path))
	return err

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
// Returns -1 if error is encountered.
func latestVersion(path string) (int, error) {
	//get all files in the path
	entries, err := os.ReadDir(filepath.Join(".", znodeDir, path))
	if err != nil {
		return -1, err
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
		//should error if encounter a filename that is not a number
		if err != nil {
			return -1, err
		}

		if version > latest {
			latest = version
		}
	}
	return latest, nil
}

// getZnode returns a znode with the specified path.
// If not in cache will check if in storage and add to cache.
func getZnode(znodeCache map[string]*ZNode, path string) (*ZNode, error) {
	if znode, exists := znodeCache[path]; exists {
		return znode, nil
	}
	if existsZnode(znodeCache, path) {
		version, err := latestVersion(path)
		if err != nil {
			return nil, err
		}
		znode := &ZNode{
			Path:       path,
			Version:    version,
			Ephermeral: false, //This might be an issue, tentatively assuming if znode instance is not in cache but in storage, it is not ephermeral
		}
		znodeCache[path] = znode
		return znode, nil
	}
	//return nil if znode does not exist
	return nil, errors.New("znode does not exist")
}

// children returns the children of a znode.
// Will be empty if znode has no children.
func childrenZnode(path string) ([]string, error) {
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

// seqname returns a name for a znode with a sequence number.
// Will return the modified path.
// Meant to be used to create znodes with the sequential flag set.
func seqname(path string) (string, error) {
	parentpath := filepath.Dir(path)
	filename := filepath.Base(path)
	siblings, err := childrenZnode(parentpath)
	if err != nil {
		return "", err
	}
	maxseq := 0
	for _, sibling := range siblings {
		index := strings.LastIndex(sibling, "_")
		if index == -1 {
			continue
		}
		if sibling[:index] == filename {
			seq, err := strconv.Atoi(sibling[index+1:])
			if err != nil {
				//skip if not a number
				continue
			}
			if seq > maxseq {
				maxseq = seq
			}
		}
	}
	newfilename := filename + "_" + strconv.Itoa(maxseq+1)

	return filepath.Join(parentpath, newfilename), nil
}
