package znode

// functions meant to be used by leader server node to check conditions before any write operations

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
)

// Check if a write request is valid.
// check error to see if request is valid, if not, will return error message
// will return nil if request is invalid, otherwise will return updated request to broadcast
func Check(cache *ZNodeCache, data []byte) ([]byte, error) {
	req := &write_request{}
	err := json.Unmarshal(data, req)
	if err != nil {
		return nil, err
	}

	switch req.Request {

	case "create":
		//if seq, modify path to include seq number
		if req.Znode.Sequential {
			err = seqname(cache, &req.Znode)
			if err != nil {
				return nil, err
			}
		}

		err = check_create(cache, &req.Znode)
		if err != nil {
			return nil, err
		}

		//update version number in request (and name if seq)
		updateddata, err := json.Marshal(&write_request{Request: "create", Znode: req.Znode})
		if err != nil {
			return nil, err
		}

		return updateddata, nil
	case "setdata":
		err = check_update(cache, &req.Znode)
		if err != nil {
			return nil, err
		}

		//update version number in request
		updateddata, err := json.Marshal(&write_request{Request: "setdata", Znode: req.Znode})
		if err != nil {
			return nil, err
		}

		return updateddata, nil
	case "delete":
		err = check_delete(cache, &req.Znode)
		if err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, &InvalidRequestError{"Invalid request: " + req.Request}
	}
}

// seqname updates the znode path to include a sequence number
// Meant to be used to create znodes with the sequential flag set.
func seqname(cache *ZNodeCache, znode *ZNode) error {
	//TODO implement
	parentpath := filepath.Dir(znode.Path)
	filename := filepath.Base(znode.Path)
	siblings := cache.cache[parentpath].Children

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
	//update path
	znode.Path = filepath.Join(parentpath, newfilename)
	return nil
}

// check_create checks for conditions that must be met before creating a znode
func check_create(cache *ZNodeCache, znode *ZNode) error {
	//check if znode exists already in cache
	if _, ok := cache.cache[znode.Path]; ok {
		return &ExistsError{"znode already exists"}
	}

	//check if parent znode exists, if not, znode cannot be created
	parentpath := filepath.Dir(znode.Path)
	if _, ok := cache.cache[parentpath]; !ok {
		return &ExistsError{"parent znode does not exist"}
	}

	//check if parent node is ephemeral
	if cache.cache[parentpath].Ephemeral {
		return &InvalidRequestError{"parent znode is ephemeral"}
	}

	//check version? should not be an issue, but leaving this here for now
	//version number for create should be 0 as write will increment to 1
	if znode.Version != 0 {
		return &VersionError{"version number should be 0 for create"}
	}

	//add znode to cache upon successful check
	znode.Version++ //write will increment version number too, but that should be fine as it is not taking in this znode instance but rather the one from the request
	cache.cache[znode.Path] = znode
	//add znode to parent's children
	cache.cache[parentpath].Children = append(cache.cache[parentpath].Children, filepath.Base(znode.Path))

	return nil
}

// check_update checks for conditions that must be met before overwriting
func check_update(cache *ZNodeCache, znode *ZNode) error {
	//check znode exists
	if _, ok := cache.cache[znode.Path]; !ok {
		return &ExistsError{"znode already exists"}
	}

	//check version match if not -1
	if znode.Version != -1 {
		if znode.Version != cache.cache[znode.Path].Version {
			return &VersionError{"version number does not match latest version"}
		}
	}

	//update cache znode, only update data and version, other fields should not be updated
	cache.cache[znode.Path].Data = znode.Data
	cache.cache[znode.Path].Version++

	return nil
}

// check_delete checks for conditions that must be met before deleting a znode
// TODO does not check for children, deleting is currently recursive, all children will be deleted
func check_delete(cache *ZNodeCache, znode *ZNode) error {
	//check znode exists
	if _, ok := cache.cache[znode.Path]; !ok {
		return &ExistsError{"znode does not exists"}
	}

	//check version match if not -1
	if znode.Version != -1 {
		if znode.Version != cache.cache[znode.Path].Version {
			return &VersionError{"version number does not match latest version"}
		}
	}

	//remove znode from parent's children
	//TODO better way to do this?
	parentpath := filepath.Dir(znode.Path)
	children := cache.cache[parentpath].Children
	for i, child := range children {
		if child == filepath.Base(znode.Path) {
			children = append(children[:i], children[i+1:]...)
			break
		}
	}
	cache.cache[parentpath].Children = children

	//remove znode from cache
	delete(cache.cache, znode.Path)

	return nil
}
