package znode

import (
	"path/filepath"
)

// ZNodeCache is an in-memory cache for ZNodes
// Field cache is only accessible by the package to avoid potential desyncs between cache and memory
// Used only by leader for checks
type ZNodeCache struct {
	cache map[string]*ZNode
}

var znodecache *ZNodeCache

// Init_znode_cache initializes the cache with all znodes in storage
func Init_znode_cache() error {
	znodecache = &ZNodeCache{cache: make(map[string]*ZNode)}
	// ensure base znode exists in storage (for checking children)
	if !existsZnode(".") {
		base_znode, session_znode, err := init_base_znode()
		if err != nil {
			return err
		}
		znodecache.cache[base_znode.Path] = base_znode
		// manually add sessiondir to cache to avoid it being counted as a child of base znode
		znodecache.cache[session_znode.Path] = session_znode
	} else {
		// if base znode exists, add to cache
		base_znode, err := GetData(".")
		if err != nil {
			return err
		}
		znodecache.cache[base_znode.Path] = base_znode
		session_znode, err := GetData(sessionDir)
		if err != nil {
			return err
		}
		znodecache.cache[session_znode.Path] = session_znode
	}

	// populate cache with all children of znodeDir
	err := populate_cache_layer(".")
	if err != nil {
		return err
	}
	// populate cache with all children of sessionDir
	err = populate_cache_layer(sessionDir)
	if err != nil {
		return err
	}

	return nil
}

// init_base_znode initializes the base znode in storage
// also inits sessionDir znode, which stores session znodes
// should exist unless system is brand new
func init_base_znode() (*ZNode, *ZNode, error) {
	base_znode := &ZNode{
		Path: ".",
		Data: []byte("This is the base znode, it should not be deleted. This is used to store info of children of the root znode."),
	}
	err := write_op(base_znode)
	if err != nil {
		return nil, nil, err
	}

	session_znode := &ZNode{
		Path: sessionDir,
		Data: []byte("This is the session znode, it should not be deleted. This is used to store session info."),
	}
	err = write_op(session_znode)
	if err != nil {
		return nil, nil, err
	}
	return base_znode, session_znode, nil
}

// populate_cache_layer populates the cache with all children of the znode with specified path
// recursively calls itself on all children
func populate_cache_layer(path string) error {
	children, err := GetChildren(path)
	if err != nil {
		return &CriticalError{"Critical error! Error populating cache layer: " + err.Error()}
	}
	// base case: No children
	if len(children) == 0 {
		return nil
	}

	// Recursive case: get all child znodes and add to cache, then call populate_cache_layer on each child
	for _, child := range children {
		childpath := filepath.Join(path, child)
		znode, err := GetData(childpath)
		if err != nil {
			return err
		}
		znodecache.cache[childpath] = znode
		err = populate_cache_layer(childpath)
		if err != nil {
			return err
		}
	}
	return nil
}

// Print_znode_cache prints the contents of the cache
// Used for debugging
func Print_znode_cache() {
	for _, znode := range znodecache.cache {
		PrintZnode(znode)
	}
}
