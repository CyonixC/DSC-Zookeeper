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
func Check(data []byte) ([]byte, error) {
	err := check_znode_init()
	if err != nil {
		return nil, err
	}

	req := &write_request{}
	err = json.Unmarshal(data, req)
	if err != nil {
		return nil, err
	}

	switch req.Request {

	case "create":
		//if seq, modify path to include seq number
		if req.Znodes[0].Sequential {
			err = seqname(&req.Znodes[0])
			if err != nil {
				return nil, err
			}
		}

		err = check_create(&req.Znodes[0])
		if err != nil {
			return nil, err
		}

		//update version number in request (and name if seq)
		updateddata, err := json.Marshal(&write_request{Request: "create", Znodes: req.Znodes})
		if err != nil {
			return nil, err
		}

		return updateddata, nil
	case "setdata":
		err = check_update(&req.Znodes[0])
		if err != nil {
			return nil, err
		}

		//update version number in request
		updateddata, err := json.Marshal(&write_request{Request: "setdata", Znodes: req.Znodes})
		if err != nil {
			return nil, err
		}

		return updateddata, nil
	case "delete":
		err = check_delete(&req.Znodes[0])
		if err != nil {
			return nil, err
		}
		return data, nil
	case "delete_session":
		err = check_delete_session(&req.Znodes[0])
		if err != nil {
			return nil, err
		}
		return data, nil

	case "sync":
		return data, nil

	case "watch_trigger":
		//effectively multiple setdata operations
		for _, znode := range req.Znodes {
			err = check_update(&znode)
			if err != nil {
				return nil, err
			}
		}
		updateddata, err := json.Marshal(&write_request{Request: "watch_trigger", Znodes: req.Znodes})
		if err != nil {
			return nil, err
		}

		return updateddata, nil
	default:
		return nil, &InvalidRequestError{"Invalid request: " + req.Request}
	}
}

// seqname updates the znode path to include a sequence number
// Meant to be used to create znodes with the sequential flag set.
func seqname(znode *ZNode) error {
	parentpath := filepath.Dir(znode.Path)
	filename := filepath.Base(znode.Path)
	siblings := znodecache[parentpath].Children

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
func check_create(znode *ZNode) error {
	//check if znode exists already in cache
	if _, ok := znodecache[znode.Path]; ok {
		return &ExistsError{"znode already exists"}
	}

	//check if parent znode exists, if not, znode cannot be created
	parentpath := filepath.Dir(znode.Path)
	if _, ok := znodecache[parentpath]; !ok {
		return &ExistsError{"parent znode does not exist"}
	}

	//check if parent node is ephemeral
	if znodecache[parentpath].Ephemeral != "" {
		return &InvalidRequestError{"parent znode is ephemeral"}
	}

	//check version? should not be an issue, but leaving this here for now
	//version number for create should be 0 as write will increment to 1
	if znode.Version != 0 {
		return &VersionError{"version number should be 0 for create"}
	}

	//if ephemeral, check session znode exists
	if znode.Ephemeral != "" {
		session_znode, ok := znodecache[filepath.Join(sessionDir, znode.Ephemeral)]
		if !ok {
			return &ExistsError{"session znode does not exist"}
		}
		// update session znode in cache
		session := &Session{}
		err := json.Unmarshal(session_znode.Data, session)
		if err != nil {
			return &CriticalError{"Crictial Error! session znode data is invalid"}
		}
		session.EphemeralNodes = append(session.EphemeralNodes, znode.Path)
		sessiondata, err := json.Marshal(session)
		if err != nil {
			return err
		}
		session_znode.Version++
		session_znode.Data = sessiondata
	}

	//add znode to cache upon successful check
	znode.Version++ //write will increment version number too, but that should be fine as it is not taking in this znode instance but rather the one from the request
	znodecache[znode.Path] = znode
	//add znode to parent's children
	znodecache[parentpath].Children = append(znodecache[parentpath].Children, filepath.Base(znode.Path))

	return nil
}

// check_update checks for conditions that must be met before overwriting
func check_update(znode *ZNode) error {
	//check znode exists
	if _, ok := znodecache[znode.Path]; !ok {
		return &ExistsError{"znode already exists"}
	}

	//check version match if not -1
	if znode.Version != -1 {
		if znode.Version != znodecache[znode.Path].Version {
			return &VersionError{"version number does not match latest version"}
		}
	}

	znode.Version++
	//update cache znode, only update data and version, other fields should not be updated
	znodecache[znode.Path].Data = znode.Data
	znodecache[znode.Path].Version++

	return nil
}

// check_delete checks for conditions that must be met before deleting a znode
// TODO does not check for children, deleting is currently recursive, all children will be deleted
func check_delete(znode *ZNode) error {
	//check znode exists
	if _, ok := znodecache[znode.Path]; !ok {
		return &ExistsError{"znode does not exists"}
	}

	//check version match if not -1
	if znode.Version != -1 {
		if znode.Version != znodecache[znode.Path].Version {
			return &VersionError{"version number does not match latest version"}
		}
	}

	//remove znode from parent's children
	//TODO better way to do this?
	parentpath := filepath.Dir(znode.Path)
	children := znodecache[parentpath].Children
	for i, child := range children {
		if child == filepath.Base(znode.Path) {
			children = append(children[:i], children[i+1:]...)
			break
		}
	}
	znodecache[parentpath].Children = children

	//remove znode from cache
	delete(znodecache, znode.Path)

	return nil
}

func check_delete_session(znode *ZNode) error {
	//check session znode exists
	session_znode, err := GetData(znode.Path)
	if err != nil {
		return err
	}

	//remove all ephemeral nodes from cache
	session_data := &Session{}
	err = json.Unmarshal(session_znode.Data, session_data)
	if err != nil {
		return &CriticalError{"Crictial Error! session znode data is invalid"}
	}
	for _, path := range session_data.EphemeralNodes {
		//remove znode from parent's children
		parentpath := filepath.Dir(path)
		children := znodecache[parentpath].Children
		for i, child := range children {
			if child == filepath.Base(path) {
				children = append(children[:i], children[i+1:]...)
				break
			}
		}
		znodecache[parentpath].Children = children
		//remove znode from cache
		delete(znodecache, path)
	}

	//remove session znode from cache
	delete(znodecache, znode.Path)

	return nil
}
