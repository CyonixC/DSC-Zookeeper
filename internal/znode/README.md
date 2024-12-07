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
Refer to znodetest/main.go for example usage of logic flow for each client api call.


## Initialisation

*For leader server ONLY*
Use `Init_znode_cache()` to initialise the in-memory cache of znode.
This cache is used for checking validity of requests by the leader
Will automatically populate the cache with znodes from local storage, otherwise init directory struct for fresh system.
Technically can be used to sync in-memory cache to local storage (re-init)
`Print_znode_cache()` is also available for debugging purposes

```go
var err error
err := znode.Init_znode_cache()
znode.Print_znode_cache()
```

Use `Init_watch_cache()` to create a local cache to track watch flags.
Each server only caches flags for the sessions they handle.
But session watch info is propogated through a write request (refer to Handling Watch Flags below)
Technically can be used to reset watch cache. Use `Update_watch_cache()` to repopulate the cache.
`Print_watch_cache()` is also available for debugging purposes

```go
var err error
znode.Init_watch_cache()
znode.Print_watch_cache
```

*When establishing an existing session*
`Update_watch_cache()` is used to populate the watch_cache from local storage.
This is meant for when a client with an existing session establishes a new connection with the server.
Will only add the watchflags for the specified sessions.
Will also check to ensure versions match local, if any mismatch will return paths of mismatched (to inform client) and special write request to update session znode. (Triggered flags)

```go
var sessionid string
var request []byte
var paths []string
var err error
request, paths, err := znode.Update_watch_cache(sessionid)
```

## Encoding Write Requests

This package provides functions corresponding the write requests in the client API.
These functions packages the request as []byte to be sent by the Proposal package.

`Encode_create()` corresponds to the create() client API
```go
var path string
var data []byte
var ephemeral bool
var sequential bool
var sessionid string
var request []byte
var err error
request, err := znode.Encode_create(path, data, ephemeral, sequential, sessionid)
```

`Encode_delete()` corresponds to the delete() client API
```go
var path string
var version int
var request []byte
var err error
request, err := znode.Encode_delete(path, version)
```

`Encode_setdata()` corresponds to the setData() client API
```go
var path string
var data []byte
var version int
var request []byte
var err error
request, err := znode.Encode_setdata(path, data, version)
```


`Encode_sync()` corresponds to the sync() client API
Creates empty write request that will autopass the check by server
Write() does not currently handle this, server that made sync request to detect and complete sync operation
```go
var request []byte
var err error
request, err := znode.Encode_sync()
```

## Processing Write Requests

These functions take in write requests encoded by the above functions.

*For leader server ONLY*
`Check()` checks validity of write request. 
This checks for conditions such as valid paths etc and is meant for the leader server node to check create, setdata and delete requests. 
Checks against in-memory ZNodeCache, NOT local storage
Will update the cache and the request (i.e. filename for seq flag/version), but will not write yet
Returns the updated request to be broadcasted to all servers

```go
var request []byte
var updated_request []byte
var err error
updated_request, err := znode.Check(request)
```

`Write()` writes a znode to local storage. 
This is meant for servers to call upon receiving commit message from leader. 
Returns array of paths of all modified znodes, meant to be used to check watch flags using `Check_watch()`, refer to Handling Watch Flags below
This array will have only 1 path for the standard create, setdata and delete, the exception being delete_session (i.e.deleting multiple ephermeral notes)
Use filepath.Base to get name for create to return to client

```go
var req []byte
var paths []string
var err error
paths, err := znode.Write(req)
```

## Handling Read Requests

This package provides functions corresponding the read requests in the client API.
These functions read data from local storage.
These functions do NOT handle the watch flag, refer to below.

`Exists()` corresponds to the exists() client API

```go
var path string
var exists bool
exists = Exists(path)
```

`GetData()` corresponds to the getData() client API
Returns a ZNode struct, described below

```go
var path string
var znode ZNode
var err error
znode, err = GetData(path)
```

`GetChildren()` corresponds to the getChildren() client API
Will return an empty array if no children

```go
var path string
var children []string
var err error
children, err = GetChildren(path)
```


## Handling Watch Flags

Use `Encode_watch()` to handle any watch flags that come with read requests.
It will update the watch cache and return a write request to be sent to the leader to propogate the watch flag.
The watch flag if true generates a write request to add watch flag, false removes.
By default when handling watch flags should be true, false is meants for use of this func by `Check_watch()` below

```go
var sessionid string
var path string
var watch bool
var request []byte
var err error
request, err = Encode_watch(sessionid, path, watch)
```

Use `Check_watch()` after commiting a write request with `Write()`.
This checks the cache for all sessions watching the updated znode.
It will return an array of session ids, these are all the sessions this server handles and will need to notify that there has been a change to the watched znode.
It will also return a request to send to the leader to propogate the updated session info (remove triggered watch flag).

paths should be returned by `Write()`.
```go
var paths []string
var request []byte
var sessions []string
var err error
request, sessions, err = Encode_watch(paths)
```

## Sessions

Updating of session information(ephermeral/watch flags) is done by other functions above.
The only things that need to be explicitly done is create a session znode upon establishing a new session,
and deleting a session znode and all its ephermeral nodes upon closing session or timeout

Use `Exists_session()` to check if a session znode already exists for the session.
If it does, this means the client already has an ongoing session and somehow lost connection to its previous server.
This means you will need to use `Update_watch_cache()` to add its watch flags to this server's watch cache

```go
var sessionid string
var exists bool
exists= Exists_session(sessionid)
```

Use `Get_sessions()` to get a list of (session id of ) active sessions
Meant for new leader to start timers upon being elected
Just a wrapper around the `GetChildren()` function

```go
var session_ids []string
var err error
session_ids, err = Get_sessions()
```

`Encode_create_sessions()` creates a write request to be sent to the leader.
This initialises the session znode and must be used to establish a new session.
Before using this, check to ensure it is a new session or it will error.

```go
var sessionid string
var timeout int
var request []byte
var err error
request, err= Encode_create_session(sessionid, timeout)
```

`Encode_delete_session()` creates a special write request that both deletes the session znode and all associated ephemeral nodes.

```go
var sessionid string
var request []byte
var err error
request, err= Encode_delete_session(sessionid)
```


## Znode format
Znode struct used mostly internally in this package but also returned by `GetData()`
`PrintZnode()` is available for debugging
path structure: ./znodeDir/znodePath/znodename.json
currently not storing watch flags otherwise will be returned together with `GetData()`

```go
type ZNode struct {
	Path       string 
	Data       []byte
	Version    int
	Ephemeral  string 
	Sequential bool
	Children   []string
}
var znode ZNode
PrintZnode(znode)
```

## Custom Errors
The znode package provides custom errors to allow error handling using errors.As or type checking.
Alternatively print(err.Error()) to observe detailed error message.

`InvalidRequestError`: When write request has an invalid type, can be thrown by `Check()` and `Write()`

`VersionError`: When provided version does not match latest version locally, thrown during `Check()`

`ExistsError`: When a znode exists when it should not (e.g. during create) or vice versa (e.g. reading/deleting/updating/missing parent)

`InitError`: When attempting to use znodecache or watchcache without initialising them. Only checks for at least 1 initialisation.

`CriticalError`: Only thrown when there is an error caused by a bug in this package, please inform Darren if encountered