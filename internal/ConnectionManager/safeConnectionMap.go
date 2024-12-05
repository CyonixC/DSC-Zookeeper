package connectionManager

import (
	"net"
	"sync"
)

type SafeConnectionMap struct {
	sync.RWMutex
	connMap map[string]net.Conn
}

func (smap *SafeConnectionMap) store(key string, val net.Conn) {
	smap.Lock()
	smap.Unlock()
	smap.connMap[key] = val
}

func (smap *SafeConnectionMap) load(key string) (net.Conn, bool) {
	smap.RLock()
	defer smap.RUnlock()
	val, ok := smap.connMap[key]
	return val, ok
}

func (smap *SafeConnectionMap) remove(key string) {
	smap.Lock()
	defer smap.Unlock()
	delete(smap.connMap, key)
}
