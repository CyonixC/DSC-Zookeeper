package znode

//This file defines utils for watch flag functionality

import (
	"encoding/json"
	"path/filepath"
)

//TODO implement safety to prevent session znodes from being added to watchlist
//TODO implement safety to ensure watchcache was initialised before use

// map of znode paths to sessions watching them
// watch flags are used to signal to clients that a znode has been modified
// watch flags are cleared after being read
var watchcache map[string][]string

// Creates an empty watch cache
// Use Update_watch_cache to populate the cache
func Init_watch_cache() {
	watchcache = make(map[string][]string)
}

// Update_watch_cache updates the watch cache with the watchlist of a session
// Used either to init cache or when picking up an existing session
func Update_watch_cache(sessionid string) error {
	sessionpath := filepath.Join(sessionDir, sessionid)
	session_znode, err := GetData(sessionpath)
	if err != nil {
		return err
	}
	session_data := &Session{}
	err = json.Unmarshal(session_znode.Data, session_data)
	if err != nil {
		return &CriticalError{"Crictial Error! session znode data is invalid"}
	}
	for _, path := range session_data.Watchlist {
		watchcache[path] = append(watchcache[path], sessionid)
	}
	return nil
}

// Encode_watch is a wrapper that calls Encode_setdata to update a session's watchlist
// set watch to true to add watch flag, false to remove
// will add session to watch cache if watch is true, but does nothing if false
func Encode_watch(sessionid string, path string, watch bool) ([]byte, error) {
	//Get session znode locally and update its watchlist
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
	if watch {
		session_data.Watchlist = append(session_data.Watchlist, path)
		//add session to watch cache
		watchcache[path] = append(watchcache[path], sessionid)
	} else {
		for i, watchpath := range session_data.Watchlist {
			if watchpath == path {
				session_data.Watchlist = append(session_data.Watchlist[:i], session_data.Watchlist[i+1:]...)
				break
			}
		}
	}
	session_znode.Data, err = json.Marshal(session_data)
	if err != nil {
		return nil, err
	}

	//propagate info
	return Encode_setdata(sessionpath, session_znode.Data, session_znode.Version)
}

// Check_watch checks the watch cache for any sessions watching the given paths
// Returns requests to update watchlist for each session
// Returns list of sessions that were watching the paths
// Clears watchlist for each path
// TODO figure out better system for storing watch info, avoid so many write requests
func Check_watch(paths []string) ([][]byte, []string, error) {
	var reqs [][]byte
	var sessions []string
	for _, path := range paths {
		var temp_sessions []string
		//get all sessions watching this path
		if len(watchcache[path]) > 0 {
			temp_sessions = append(temp_sessions, watchcache[path]...)
		}
		//generate requests to update watchlist for each session
		for _, sessionid := range temp_sessions {
			req, err := Encode_watch(sessionid, path, false)
			if err != nil {
				return nil, nil, err
			}
			reqs = append(reqs, req)
		}
		sessions = append(sessions, temp_sessions...)
		//clear path from watch cache
		delete(watchcache, path)
	}
	return reqs, sessions, nil
}

// Print_watch_cache prints the watch cache for debugging purposes
func Print_watch_cache() {
	for path, sessions := range watchcache {
		println("Path: ", path)
		for _, session := range sessions {
			println("Session: ", session)
		}
	}
}
