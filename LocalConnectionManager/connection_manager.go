package localconnectionmanager

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type SafeConnectionMap struct {
	sync.RWMutex
	connMap map[string]chan NetworkMessage
}
type NetworkMessage struct {
	Remote  net.Addr
	Message []byte
}

var cmap = SafeConnectionMap{connMap: make(map[string]chan NetworkMessage)}

func (smap *SafeConnectionMap) store(key net.Addr, val chan NetworkMessage) {
	smap.Lock()
	defer smap.Unlock()
	smap.connMap[key.String()] = val
}

func (smap *SafeConnectionMap) load(key net.Addr) (chan NetworkMessage, bool) {
	smap.RLock()
	defer smap.RUnlock()
	val, ok := smap.connMap[key.String()]
	return val, ok
}

func (smap *SafeConnectionMap) loadOrStore(key net.Addr, val chan NetworkMessage) (chan NetworkMessage, bool) {
	oldval, ok := smap.load(key)
	if !ok {
		smap.store(key, val)
		return val, false
	} else {
		return oldval, true
	}
}

func Init(address net.Addr) (receive_channel chan NetworkMessage) {
	receive_channel = make(chan NetworkMessage)
	cmap.store(address, receive_channel)
	cmap.RLock()
	for a, b := range cmap.connMap {
		fmt.Println(a, b)
	}
	cmap.RUnlock()
	return
}

func SendMessage(toSend NetworkMessage, selfIp net.Addr, success chan bool) {
	ch, _ := cmap.load(toSend.Remote)
	toSend.Remote = selfIp
	defer func() {
		// Fail on closed channel
		if r := recover(); r != nil {
			success <- false
		}
	}()
	timer := time.NewTimer(time.Duration(sendMessageTimeoutSeconds) * time.Second)
	select {
	case ch <- toSend:
		success <- true
	case <-timer.C:
		success <- false
	}
}

func Broadcast(toSend []byte, selfIp net.Addr) {
	cmap.RLock()
	defer cmap.RUnlock()
	for _, ch := range cmap.connMap {
		go func() { ch <- NetworkMessage{selfIp, toSend} }()
	}
}
