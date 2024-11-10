package znode

// functions in this file correspond to the client API read operations of the same name

// Exists checks if a znode exists.
func Exists(path string) bool {
	return existsZnode(path)
}

// GetData returns locally stored version of a znode with the specified path.
// Returns an error if the znode does not exist.
func GetData(path string) (*ZNode, error) {
	if existsZnode(path) {
		znode := &ZNode{
			Path: path,
		}
		//read the znode data
		err := readZnode(znode)
		if err != nil {
			return nil, err
		}

		return znode, nil
	}

	//return nil if znode does not exist
	return nil, &ExistsError{"znode does not exist"}
}

// children returns the children of a znode.
// Will be empty if znode has no children.
func GetChildren(path string) ([]string, error) {

	if existsZnode(path) {
		znode, err := GetData(path)
		if err != nil {
			return nil, err
		}
		return znode.Children, nil
	}
	return nil, &ExistsError{"znode does not exist"}
}
