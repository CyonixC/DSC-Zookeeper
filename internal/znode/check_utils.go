package znode

// functions meant to be used by leader server node to check conditions before any write operations

import (
	"encoding/json"
	"path/filepath"
)

// Check if a write request is valid.
// check error for reason if request is invalid.
func Check(data []byte) (bool, error) {
	req := &write_request{}
	err := json.Unmarshal(data, req)
	if err != nil {
		return false, err
	}

	switch req.Request {

	case "create":
		//if seq, modify path to include seq number
		if req.Sequential {
			_, err = seqname(&req.Znode)
			if err != nil {
				return false, err
			}
		}
		return check_create(&req.Znode)
	case "setdata":
		return check_update(&req.Znode)
	case "delete":
		return check_update(&req.Znode)
	default:
		return false, &InvalidRequestError{"Invalid request: " + req.Request}
	}
}

// check_create checks for conditions that must be met before creating a znode
func check_create(znode *ZNode) (bool, error) {
	//check if znode exists already
	if existsZnode(znode.Path) {
		return false, &ExistsError{"znode already exists"}
	}

	//check if parent znode exists, if not, znode cannot be created
	parentpath := filepath.Dir(znode.Path)
	if parentpath != "." && !existsZnode(parentpath) {
		return false, &ExistsError{"parent znode does not exist"}
	}

	//TODO check if parent node is ephemeral

	//check version? should not be an issue, but leaving this here for now
	//version number for create should be 0 as write will increment to 1
	if znode.Version != 0 {
		return false, &VersionError{"version number should be 0 for create"}
	}

	return true, nil
}

// check_update checks for conditions that must be met before overwriting or deleting a znode
// TODO does not check for children, deleting is currently recursive, all children will be deleted
func check_update(znode *ZNode) (bool, error) {
	//check znode exists
	if !existsZnode(znode.Path) {
		return false, &ExistsError{"znode does not exist"}
	}

	//check version match if not -1
	if znode.Version != -1 {
		ver, err := latestVersion(znode.Path)
		if err != nil {
			return false, err
		}
		if znode.Version != ver {
			return false, &VersionError{"version number does not match latest version"}
		}
	}

	return true, nil
}
