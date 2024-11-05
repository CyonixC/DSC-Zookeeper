# znode
This package is meant for servernodes to handle and store data in znode format.

## Usage

Import this file in your code:
```go
import (
    // other packages...
    "local/zookeeper/internal/znode"
)
```
Refer to znodetest/main.go for example usage

## Available Functions

`Check()` checks validity of write request. 
This checks for conditions such as valid paths etc and is meant for the leader server node to check create, setdata and delete requests. 
Takes in []Byte and deserialises into write request format described below.

```go
var req []byte
var check bool
var err error
check, err := znode.Check(req)
```

`Write()` writes a znode to local storage. 
This is meant for servers to call upon receiving commit message from leader. 
Returns name for create request, empty string otherwise.
Takes in []Byte and deserialises into write request format described below. 

```go
var req []byte
var name string
var err error
name, err := znode.Write(req)
```

`Exists()` checks if a znode exists locally.
Corresponds to client api call of the same name

```go
var path string
var exists bool
exists = Exists(path)
```

`GetData()` retrieves a znode from local storage.
Returns a ZNode struct, described below
Corresponds to client api call of the same name

```go
var path string
var znode ZNode
var err error
znode, err = GetData(path)
```

`GetChildren()` retrieves the children of a znode from local storage.
Will return an empty array if no children
Corresponds to client api call of the same name

```go
var path string
var children []string
var err error
children, err = GetChildren(path)
```

## Message format
The format of write requests meant to be passed to or received from proposals is as follows:
```go
type write_request struct {
	Request    string //"create", "setdata", "delete" only
	Znode      ZNode
	Ephemeral  bool
	Sequential bool
}
```

To ease encoding of write request, `Encode_write_request()` is provided:
*TO BE MODIFIED DEPENDING ON CLIENT API IMPLEMENTATION
```go
var request string 
var path string
var data []byte
var version int
var ephemeral bool
var sequential bool
req, err := znode.Encode_write_request(request, path, data, version, ephemeral, sequential)
```

## Znode format
Znode struct used mostly internally in this package but also returned by `GetData()`
*MAY BE MODIFIED DEPENDING ON FLAG IMPLEMENTATION

```go
type ZNode struct {
	Path    string 
	Data    []byte
	Version int
}
```

## Custom Errors
The znode package provides custom errors to allow error handling using errors.As or type checking.

`InvalidRequestError`: When write request has an invalid type, can be thrown by `Check()` and `Write()`

`VersionError`: When provided version does not match latest version locally, thrown during `Check()`

`ExistsError`: When a znode exists when it should not (e.g. during create) or vice versa (e.g. reading/deleting/updating/missing parent)