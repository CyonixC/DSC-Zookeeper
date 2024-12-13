package znode

//This file defines utils for watch flag functionality

import (
	"encoding/json"
	"fmt"
	"local/zookeeper/internal/logger"
	"path/filepath"
)

//TODO implement safety to prevent session znodes from being added to watchlist

// map of znode paths to sessions watching them
// watch flags are used to signal to clients that a znode has been modified
// watch flags are cleared after being readd
var watchcache map[string][]string
var watchinit bool = false

// Creates an empty watch cache
// Use Update_watch_cache to populate the cache
// TODO may need lock?
func Init_watch_cache() {
	watchcache = make(map[string][]string)
	watchinit = true
}

// Update_watch_cache updates the watch cache with the watchlist of a session
// Used either to init cache or when picking up an existing session
// Will also check to ensure versions match local, if any mismatch will return paths of mismatched (to inform client) and special write request to update session znode. (Triggered flags)
func Update_watch_cache(sessionid string) ([]byte, []string, error) {
	err := check_watch_init()
	if err != nil {
		return nil, nil, err
	}
	sessionpath := filepath.Join(sessionDir, sessionid)
	session_znode, err := GetData(sessionpath)
	if err != nil {
		return nil, nil, err
	}
	session_data := &Session{}
	err = json.Unmarshal(session_znode.Data, session_data)
	if err != nil {
		return nil, nil, &CriticalError{"Crictial Error! session znode data is invalid"}
	}

	paths := []string{}
	updated_watchlist := []string{}
	for i, path := range session_data.Watchlist {
		//get latest version of znode to watch
		znode, err := GetData(path)
		if err != nil {
			return nil, nil, err
		}
		//add to cache if version matches
		if znode.Version == session_data.Versionlist[i] {
			watchcache[path] = append(watchcache[path], sessionid)
			updated_watchlist = append(updated_watchlist, path)
		} else {
			//else add path to list of paths to update client and remove from watchlist
			paths = append(paths, path)
		}
	}
	if len(updated_watchlist) > 0 {
		session_data.Watchlist = updated_watchlist
		session_znode.Data, err = json.Marshal(session_data)
		if err != nil {
			return nil, nil, err
		}
		data, err := Encode_setdata(sessionpath, session_znode.Data, session_znode.Version)
		if err != nil {
			return nil, nil, err
		}
		return data, paths, nil
	} else {
		return nil, nil, nil
	}
}

// Encode_watch is a wrapper that calls Encode_setdata to update a session's watchlist
// will also add session+path to watch cache
func Encode_watch(sessionid string, path string) ([]byte, error) {
	err := check_watch_init()
	if err != nil {
		return nil, err
	}
	//Get session znode locally
	sessionpath := filepath.Join(sessionDir, sessionid)
	session_znode, err := GetData(sessionpath)
	if err != nil {
		return nil, err
	}
	session_data := &Session{}
	err = json.Unmarshal(session_znode.Data, session_data)
	if err != nil {
		return nil, &CriticalError{"Crictial Error! session znode data is invalid"}
	}
	//update watchlist
	session_data.Watchlist = append(session_data.Watchlist, path)
	logger.Debug(fmt.Sprint("Watchlist:", session_data.Watchlist))
	//get latest version of znode to watch
	znode, err := GetData(path)
	if err != nil {
		//It's fine if znode doesn't exist, attempting to watch nonexistant znode
		session_data.Versionlist = append(session_data.Versionlist, 1)
	} else {
		//update versionlist if it exists
		session_data.Versionlist = append(session_data.Versionlist, znode.Version)
	}
	//add session to watch cache
	watchcache[path] = append(watchcache[path], sessionid)
	session_znode.Data, err = json.Marshal(session_data)
	if err != nil {
		return nil, err
	}

	//propagate info
	return Encode_setdata(sessionpath, session_znode.Data, session_znode.Version)
}

// Check_watch checks the watch cache for any sessions watching the given paths
// Returns request to update watchlist for each session
// Returns list of sessions that were watching the paths
// Clears watchlist for each path
func Check_watch(paths []string) ([]byte, []string, error) {
	err := check_watch_init()
	if err != nil {
		return nil, nil, err
	}

	var znodes []ZNode
	var sessions []string
	for _, path := range paths {
		var temp_sessions []string
		//get all sessions watching this path
		if len(watchcache[path]) > 0 {
			temp_sessions = append(temp_sessions, watchcache[path]...)
		}
		//generate updated watchlist & versionlist for each znode session
		for _, sessionid := range temp_sessions {
			sessionpath := filepath.Join(sessionDir, sessionid)
			session_znode, err := GetData(sessionpath)
			if err != nil {
				return nil, nil, err
			}
			session_data := &Session{}
			err = json.Unmarshal(session_znode.Data, session_data)
			if err != nil {
				return nil, nil, &CriticalError{"Crictial Error! session znode data is invalid"}
			}
			logger.Debug(fmt.Sprint("Watchlist:", session_data.Watchlist))
			for i, watchpath := range session_data.Watchlist {
				if watchpath == path {
					session_data.Watchlist = append(session_data.Watchlist[:i], session_data.Watchlist[i+1:]...)
					session_data.Versionlist = append(session_data.Versionlist[:i], session_data.Versionlist[i+1:]...)
					session_znode.Data, err = json.Marshal(session_data)
					if err != nil {
						return nil, nil, err
					}
					break
				}
			}
			znode := &ZNode{
				Path:    sessionpath,
				Data:    session_znode.Data,
				Version: session_znode.Version,
			}
			znodes = append(znodes, *znode)
		}
		sessions = append(sessions, temp_sessions...)
		//clear path from watch cache
		delete(watchcache, path)
	}
	req := &write_request{
		Request: "watch_trigger",
		Znodes:  znodes,
	}
	logger.Debug(fmt.Sprint("Update watch session req: ", req, "Sessions: ", sessions))
	data, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	return data, sessions, nil
}

// Print_watch_cache prints the watch cache for debugging purposes
func Print_watch_cache() {
	err := check_watch_init()
	if err != nil {
		println(err.Error())
		return
	}

	for path, sessions := range watchcache {
		println("Path: ", path)
		for _, session := range sessions {
			println("Session: ", session)
		}
	}
}

func check_watch_init() error {
	if !watchinit {
		return &InitError{"Watch cache not initialised"}
	}
	return nil
}
