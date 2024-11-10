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
		//TODO somehow make the writes atomic?
		// update parent znode children
		parentpath := filepath.Dir(req.Znode.Path)
		// if is base of znodedir, ensure base znode exists in case of this being the first znode created
		if parentpath == "." {
			if !Exists(".") {
				_, _, err := init_base_znode()
				if err != nil {
					return "", err
				}
			}
		}

		parent_znode, err := GetData(parentpath)
		if err != nil {
			return "", err
		}
		parent_znode.Children = append(parent_znode.Children, filepath.Base(req.Znode.Path))
		err = write_op(parent_znode)
		if err != nil {
			return "", err
		}

		// if ephemeral, update session znode
		if req.Znode.Ephemeral != "" {
			sessionpath := filepath.Join(sessionDir, req.Znode.Ephemeral)
			session_znode, err := GetData(sessionpath)
			if err != nil {
				return "", err
			}
			session_data := &Session{}
			err = json.Unmarshal(session_znode.Data, session_data)
			if err != nil {
				return "", err
			}
			session_data.EphemeralNodes = append(session_data.EphemeralNodes, req.Znode.Path)
			session_znode.Data, err = json.Marshal(session_data)
			if err != nil {
				return "", err
			}
			err = write_op(session_znode)
			if err != nil {
				return "", err
			}
		}

		// write session update
		name := filepath.Base(req.Znode.Path)
		return name, write_op(&req.Znode)
	case "setdata":
		return "", write_op(&req.Znode)
	case "delete":
		return "", delete_op(&req.Znode)
	case "delete_session":
		// delete all ephemeral nodes
		session_znode, err := GetData(req.Znode.Path)
		if err != nil {
			return "", err
		}
		session_data := &Session{}
		err = json.Unmarshal(session_znode.Data, session_data)
		if err != nil {
			return "", &CriticalError{"Crictial Error! session znode data is invalid"}
		}
		for _, znodepath := range session_data.EphemeralNodes {
			znode, err := GetData(znodepath)
			if err != nil {
				return "", err
			}
			err = delete_op(znode)
			if err != nil {
				return "", err
			}
		}

		return "", delete_op(&req.Znode)
	default:
		return "", &InvalidRequestError{"Invalid request: " + req.Request}
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
