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
	EphemeralNodes []string
}

// Encode_create_session is a wrapper that calls Encode_create to create a session znode
func Encode_create_session(sessionid string) ([]byte, error) {
	sessionpath := filepath.Join(sessionDir, sessionid)
	session := &Session{
		Id: sessionid,
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
		Znode:   *znode,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}
