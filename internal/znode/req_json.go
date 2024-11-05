package znode

import (
	"encoding/json"
)

// standard json struct for write requests such as create, update, delete
type write_request struct {
	Request    string //create, update, delete
	Znode      ZNode
	Ephemeral  bool
	Sequential bool
}

// Encode_write_request reformats info from client api to json as []byte for proposal
func Encode_write_request(request string, path string, data []byte, version int, ephemeral bool, sequential bool) ([]byte, error) {
	//TODO modify according to client api later with only req and []byte, unpack data according to request and set default values
	//i.e. no version for create, no flags for write and delete
	znode := &ZNode{
		Path:    path,
		Data:    data,
		Version: version,
	}
	req := &write_request{
		Request:    request,
		Znode:      *znode,
		Ephemeral:  ephemeral,
		Sequential: sequential,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}
