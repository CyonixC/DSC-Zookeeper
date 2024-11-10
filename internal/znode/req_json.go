package znode

//this file defines the json struct for write requests
//it provides functions to encode the different write requests

import (
	"encoding/json"
)

// standard json struct for write requests such as create, update, delete
type write_request struct {
	Request string //create, update, delete
	Znode   ZNode
}

//TODO safety to prevent clients from interacting with session znodes directly

// Encode_create creates a request to create a znode
func Encode_create(path string, data []byte, ephemeral bool, sequential bool, sessionid string) ([]byte, error) {
	ephemeral_data := ""
	if ephemeral {
		ephemeral_data = sessionid
	}

	znode := &ZNode{
		Path:       path,
		Data:       data,
		Version:    0,
		Ephemeral:  ephemeral_data,
		Sequential: sequential,
	}
	req := &write_request{
		Request: "create",
		Znode:   *znode,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Encode_create creates a request to delete a znode
func Encode_delete(path string, version int) ([]byte, error) {
	znode := &ZNode{
		Path:    path,
		Version: version,
	}
	req := &write_request{
		Request: "delete",
		Znode:   *znode,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Encode_create creates a request to update a znode
func Encode_setdata(path string, data []byte, version int) ([]byte, error) {
	znode := &ZNode{
		Path:    path,
		Data:    data,
		Version: version,
	}
	req := &write_request{
		Request: "setdata",
		Znode:   *znode,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}
