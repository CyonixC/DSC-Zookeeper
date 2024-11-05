package localconnectionmanager

import (
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
	cmap.RUnlock()
	return
}

func sendMsgWithTimeout(sendChan chan NetworkMessage, toSend NetworkMessage, dest string, failedSend chan string) {
	defer func() {
		// Fail on closed channel
		if r := recover(); r != nil {
			failedSend <- dest
		}
	}()
	timer := time.NewTimer(time.Duration(sendMessageTimeoutSeconds) * time.Second)
	select {
	case sendChan <- toSend:
		break
	case <-timer.C:
		failedSend <- dest
	}
}

func SendMessage(toSend NetworkMessage, selfIp net.Addr, failedSend chan string) {
	dest := toSend.Remote.String()
	ch, _ := cmap.load(toSend.Remote)
	toSend.Remote = selfIp
	sendMsgWithTimeout(ch, toSend, dest, failedSend)
}

func Broadcast(toSend []byte, selfIp net.Addr, failedSends chan string) {
	cmap.RLock()
	defer cmap.RUnlock()
	for ip, ch := range cmap.connMap {
		if selfIp.String() == ip {
			continue
		}
		go sendMsgWithTimeout(ch, NetworkMessage{selfIp, toSend}, ip, failedSends)
	}
}
