package znode

// functions meant to be used by server nodes to write znode data to the filesystem upon commit

import (
	"encoding/json"
	"path/filepath"
)

// Write commits znode changes to the filesystem.
// Returns slice of paths that have been modified, or nil if error encountered.
// Will contain multiple paths only if a session znode is deleted (deleted ephermeral nodes), else will only contain 1 path.
// Use to check watchers, also used to get file name for create (filepath.Base)
// Watch_trigger will not return any paths as no need to update watchers.
func Write(data []byte) ([]string, error) {
	req := &write_request{}
	err := json.Unmarshal(data, req)
	if err != nil {
		return nil, err
	}

	switch req.Request {

	case "create":
		//TODO somehow make the writes atomic?
		// update parent znode children
		parentpath := filepath.Dir(req.Znodes[0].Path)
		// if is base of znodedir, ensure base znode exists in case of this being the first znode created
		if parentpath == "." || parentpath == sessionDir {
			if !existsZnode(".") {
				_, _, err := init_base_znode()
				if err != nil {
					return nil, err
				}
			}
		}

		parent_znode, err := GetData(parentpath)
		if err != nil {
			return nil, err
		}
		parent_znode.Children = append(parent_znode.Children, filepath.Base(req.Znodes[0].Path))
		// write parent update
		err = write_op(parent_znode)
		if err != nil {
			return nil, err
		}

		// if ephemeral, update session znode
		if req.Znodes[0].Ephemeral != "" {
			sessionpath := filepath.Join(sessionDir, req.Znodes[0].Ephemeral)
			session_znode, err := GetData(sessionpath)
			if err != nil {
				return nil, err
			}
			session_data := &Session{}
			err = json.Unmarshal(session_znode.Data, session_data)
			if err != nil {
				return nil, err
			}
			session_data.EphemeralNodes = append(session_data.EphemeralNodes, req.Znodes[0].Path)
			session_znode.Data, err = json.Marshal(session_data)
			if err != nil {
				return nil, err
			}
			session_znode.Version++
			// write session update
			err = write_op(session_znode)
			if err != nil {
				return nil, err
			}
		}
		err = write_op(&req.Znodes[0])
		if err != nil {
			return nil, err
		}

		return []string{req.Znodes[0].Path}, nil
	case "setdata":
		err = write_op(&req.Znodes[0])
		if err != nil {
			return nil, err
		}
		return []string{req.Znodes[0].Path}, nil
	case "delete":
		err = delete_op(&req.Znodes[0])
		if err != nil {
			return nil, err
		}
		return []string{req.Znodes[0].Path}, nil
	case "delete_session":
		// delete all ephemeral nodes
		session_znode, err := GetData(req.Znodes[0].Path)
		if err != nil {
			return nil, err
		}
		session_data := &Session{}
		err = json.Unmarshal(session_znode.Data, session_data)
		if err != nil {
			return nil, &CriticalError{"Crictial Error! session znode data is invalid"}
		}
		var paths []string
		for _, znodepath := range session_data.EphemeralNodes {
			znode, err := GetData(znodepath)
			if err != nil {
				return nil, err
			}
			err = delete_op(znode)
			if err != nil {
				return nil, err
			}
			paths = append(paths, znodepath)
		}
		err = delete_op(&req.Znodes[0])
		if err != nil {
			return nil, err
		}

		return paths, nil
	case "watch_trigger":
		// effectively multiple setdata operations
		for _, znode := range req.Znodes {
			err = write_op(&znode)
			if err != nil {
				return nil, err
			}
		}
		return []string{}, nil
	default:
		return nil, &InvalidRequestError{"Invalid request: " + req.Request}
	}
}

// write_op will create/overwrite znode in storage, will create znode directory if is new znode.
// Returns nil if successful.
func write_op(znode *ZNode) error {
	err := writeZNode(znode)
	if err != nil {
		return err
	}
	return nil
}

// delete_op deletes a znode in the filesystem and removes from cache.
// also removes znode from parent znode children.
// currently is a recursive delete_op, do we need to check for children?
func delete_op(znode *ZNode) error {
	// update parent znode children
	parentpath := filepath.Dir(znode.Path)
	parent_znode, err := GetData(parentpath)
	if err != nil {
		return err
	}
	for i, child := range parent_znode.Children {
		if child == filepath.Base(znode.Path) {
			parent_znode.Children = append(parent_znode.Children[:i], parent_znode.Children[i+1:]...)
			break
		}
	}
	err = write_op(parent_znode)
	if err != nil {
		return err
	}

	err = deleteZnode(znode)
	if err != nil {
		return err
	}
	return nil
}
