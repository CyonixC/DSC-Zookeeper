package znode

// functions meant to be used by server nodes to write znode data to the filesystem upon commit

import (
	"encoding/json"
	"path/filepath"
)

// Write commits znode changes to the filesystem.
// Returns the name of the znode if request is create, empty string otherwise.
func Write(data []byte) (string, error) {
	req := &write_request{}
	err := json.Unmarshal(data, req)
	if err != nil {
		return "", err
	}

	switch req.Request {

	case "create":
		name := filepath.Base(req.Znode.Path)
		//if seq, modify path to include seq number
		if req.Sequential {
			name, err = seqname(&req.Znode)
			if err != nil {
				return "", err
			}
		}
		return name, write(&req.Znode)
	case "setdata":
		return "", write(&req.Znode)
	case "delete":
		return "", delete(&req.Znode)
	default:
		return "", &InvalidRequestError{"Invalid request: " + req.Request}
	}
}

// write will create a new version of a znode in the filesystem, will create znode directory if is new znode.
// Does not overwrite existing versions, will increment version number.
// Returns nil if successful.
func write(znode *ZNode) error {
	znode.Version++
	err := writeZNode(znode)
	if err != nil {
		return err
	}
	return nil
}

// delete deletes a znode in the filesystem and removes from cache.
// currently if version matches, is a recursive delete, do we newed to check for children?
func delete(znode *ZNode) error {
	err := deleteZnode(znode)
	if err != nil {
		return err
	}
	return nil
}
