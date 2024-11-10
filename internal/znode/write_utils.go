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
		// update parent znode children
		parentpath := filepath.Dir(req.Znode.Path)
		// if is base of znodedir, ensure base znode exists in case of this being the first znode created
		if parentpath == "." {
			if !Exists(".") {
				_, err := init_base_znode()
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

		name := filepath.Base(req.Znode.Path)
		return name, write_op(&req.Znode)
	case "setdata":
		return "", write_op(&req.Znode)
	case "delete":
		// update parent znode children
		parentpath := filepath.Dir(req.Znode.Path)
		parent_znode, err := GetData(parentpath)
		if err != nil {
			return "", err
		}
		for i, child := range parent_znode.Children {
			if child == filepath.Base(req.Znode.Path) {
				parent_znode.Children = append(parent_znode.Children[:i], parent_znode.Children[i+1:]...)
				break
			}
		}
		err = write_op(parent_znode)
		if err != nil {
			return "", err
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
// currently if version matches, is a recursive delete_op, do we newed to check for children?
func delete_op(znode *ZNode) error {
	err := deleteZnode(znode)
	if err != nil {
		return err
	}
	return nil
}
