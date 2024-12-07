package znode

//this file defines the structs and functions for managing the session znode

import (
	"encoding/json"
	"path/filepath"
)

const sessionDir = "sessionDir"

// This defines the data stored in a session znode
type Session struct {
	Id             string
	Watchlist      []string
	Versionlist    []int
	EphemeralNodes []string
	Timeout        int
}

// Exists_session checks if a session znode exists in storage.
func Exists_session(sessionid string) bool {
	sessionpath := filepath.Join(sessionDir, sessionid)
	return existsZnode(sessionpath)
}

// return list of all sessionids
func Get_sessions() ([]string, error) {
	return GetChildren(sessionDir)
}

// Encode_create_session is a wrapper that calls Encode_create to create a session znode
func Encode_create_session(sessionid string, timeout int) ([]byte, error) {
	sessionpath := filepath.Join(sessionDir, sessionid)
	session := &Session{
		Id:      sessionid,
		Timeout: timeout,
	}
	sessiondata, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	return Encode_create(sessionpath, sessiondata, false, false, sessionid)
}

func Encode_delete_session(sessionid string) ([]byte, error) {
	sessionpath := filepath.Join(sessionDir, sessionid)
	znode := &ZNode{
		Path: sessionpath,
	}
	req := &write_request{
		Request: "delete_session",
		Znodes:  []ZNode{*znode},
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}
