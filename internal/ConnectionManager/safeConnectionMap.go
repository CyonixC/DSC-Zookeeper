package connectionManager

import (
	"net"
	"sync"
	"time"
)

type SafeConnectionMap struct {
	sync.RWMutex
	connMap map[string]net.Conn
}

func (smap *SafeConnectionMap) store(key string, val net.Conn) {
	smap.Lock()
	defer smap.Unlock()
	val.SetReadDeadline(time.Time{})
	val.SetWriteDeadline(time.Now().Add(time.Duration(tcpWriteTimeoutSeconds) * time.Second))
	smap.connMap[key] = val
}

func (smap *SafeConnectionMap) load(key string) (net.Conn, bool) {
	smap.RLock()
	defer smap.RUnlock()
	val, ok := smap.connMap[key]
	return val, ok
}

func (smap *SafeConnectionMap) loadOrStore(key string, val net.Conn) (net.Conn, bool) {
	oldval, ok := smap.load(key)
	if !ok {
		smap.store(key, val)
		return val, false
	} else {
		return oldval, true
	}
}
