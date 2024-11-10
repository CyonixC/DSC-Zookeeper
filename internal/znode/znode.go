package znode

//This file contains functions meant for interacting with the filesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// currently store znodes in this project's directory
// path structure: ./znodeDir/znodePath/znodename.json
// meaning working directory for server must be directory containing znodeDir (prob root of project)
// version number currently just be an integer, incremented on each write
// this allows znodes to have children (and may help with fault tolerance later?)
// also ensures children can only be created if parent znode exists
const znodeDir = "znodeDir"

type ZNode struct {
	Path       string //Path provided by client will be a relative Path in znodeDir, corresponding to znodepath above
	Data       []byte
	Version    int
	Ephemeral  string //empty string if not ephemeral, otherwise contains session id
	Sequential bool
	Children   []string
	//current implementationn avoids storing watch flag in metadata
}

// writeZNode creates/overwrites znode in the filesystem, returns nil if successful.
// Does not handle version checking and will overwrite if version exists, be sure to check version
func writeZNode(znode *ZNode) error {
	path := filepath.Join(".", znodeDir, znode.Path)
	//create dir for znode
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	//convert znode to json
	jsonData, err := json.MarshalIndent(znode, "", "  ")
	if err != nil {
		return err
	}

	//create a file in the path
	//TODO modify implementation such that will error if overwriting existing version (if we establish that should not happen)
	path = filepath.Join(path, filepath.Base(path)+".json")
	err = os.WriteFile(path, jsonData, 0644)
	return err
}

// readZnode reads a znode of from the filesystem.
// Updates the data field of the znode struct.
// Does not handle version checking.
func readZnode(znode *ZNode) error {
	//This weird split is for case where znode.Path = "."
	path := filepath.Join(".", znodeDir, znode.Path)
	path = filepath.Join(path, filepath.Base(path)+".json")
	//read the file in the path
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, znode)
	return err
}

// deleteZnode deletes a znode (version) from the filesystem, returns nil if successful.
// Does not handle version checking.
// RELIC from when file was stored as {version}.txt
// func deleteZnodeVer(znode *ZNode) error {
// 	path := filepath.Join(".", znodeDir, znode.Path, strconv.Itoa(znode.Version)+".txt")
// 	err := os.Remove(path)
// 	return err
// }

// deleteZnode deletes a znode from the filesystem, returns nil if successful.
// Does not handle version checking nor check for children znodes.
func deleteZnode(znode *ZNode) error {
	err := os.RemoveAll(filepath.Join(".", znodeDir, znode.Path))
	return err

}

// existsZnode checks if a znode with specified path exists in storage.
func existsZnode(path string) bool {
	path = filepath.Join(".", znodeDir, path)
	info, err := os.Stat(path)
	if os.IsNotExist(err) || !info.IsDir() {
		//return false if path does not exist/checking dir is prob unnecessary but just in case
		return false
	}

	//check if znode file exists
	_, err = os.Stat(filepath.Join(path, filepath.Base(path)+".json"))
	return !os.IsNotExist(err)
}

// PrintZnode prints the fields of a znode struct.
// Used for debugging.
func PrintZnode(znode *ZNode) {
	println("=====================================")
	fmt.Printf("Path: %s\n", znode.Path)
	fmt.Printf("Data: %s\n", znode.Data)
	fmt.Printf("Version: %d\n", znode.Version)
	fmt.Printf("Ephemeral: %s\n", znode.Ephemeral)
	fmt.Printf("Sequential: %v\n", znode.Sequential)
	fmt.Printf("Children: %v\n", znode.Children)
}

// latestVersion returns the latest stored version of a znode.
// Returns -1 if error is encountered.
// Does NOT update the version field of the znode struct, up to caller to do so
// RELIC from when file was stored as {version}.txt
// func latestVersion(path string) (int, error) {
// 	//get all files in the path
// 	entries, err := os.ReadDir(filepath.Join(".", znodeDir, path))
// 	if err != nil {
// 		return -1, err
// 	}

// 	//find the latest version
// 	latest := 0
// 	for _, entry := range entries {
// 		//skip directories(children znodes)
// 		if entry.IsDir() {
// 			continue
// 		}

// 		//parse the filename to get the version number
// 		version, err := strconv.Atoi(entry.Name()[:len(entry.Name())-4])
// 		//should error if encounter a filename that is not a number
// 		if err != nil {
// 			return -1, err
// 		}

// 		if version > latest {
// 			latest = version
// 		}
// 	}
// 	return latest, nil
// }

// seqname updates the znode path to include a sequence number and returns new file name
// Meant to be used to create znodes with the sequential flag set.
// RELIC from when file was stored as {version}.txt
// func seqname(znode *ZNode) (string, error) {
// 	parentpath := filepath.Dir(znode.Path)
// 	filename := filepath.Base(znode.Path)
// 	siblings, err := GetChildren(parentpath)
// 	if err != nil {
// 		return "", err
// 	}
// 	maxseq := 0
// 	for _, sibling := range siblings {
// 		index := strings.LastIndex(sibling, "_")
// 		if index == -1 {
// 			continue
// 		}
// 		if sibling[:index] == filename {
// 			seq, err := strconv.Atoi(sibling[index+1:])
// 			if err != nil {
// 				//skip if not a number
// 				continue
// 			}
// 			if seq > maxseq {
// 				maxseq = seq
// 			}
// 		}
// 	}
// 	newfilename := filename + "_" + strconv.Itoa(maxseq+1)
// 	//update path
// 	znode.Path = filepath.Join(parentpath, newfilename)

// 	return newfilename, nil
// }
