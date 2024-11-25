package proposals

// This file contains related definitions for the ZXID counter, used by the coordinator to track
// the ZXID that should be used to label its proposals.

import "sync"

type ZXIDCounter struct {
	sync.RWMutex
	epochNum uint16
	countNum uint16
}

func (cnt *ZXIDCounter) GetLatestZXID() (zxid uint32) {
	epoch, count := cnt.check()
	zxid = getZXIDAsInt(epoch, count)
	return
}

// Get the current ZXID
func (cnt *ZXIDCounter) check() (epochNum uint16, countNum uint16) {
	cnt.RLock()
	defer cnt.RUnlock()
	epochNum = cnt.epochNum
	countNum = cnt.countNum
	return
}

func (cnt *ZXIDCounter) setVals(epochNum uint16, countNum uint16) {
	cnt.Lock()
	defer cnt.Unlock()
	cnt.epochNum = epochNum
	cnt.countNum = countNum
}

// Increment the count (same coordinator, new proposal)
func (cnt *ZXIDCounter) incCount() (epochNum uint16, countNum uint16) {
	cnt.Lock()
	defer cnt.Unlock()
	cnt.countNum++
	epochNum = cnt.epochNum
	countNum = cnt.countNum
	return
}

// Increment the epoch (new coordinator)
func (cnt *ZXIDCounter) incEpoch() (epochNum uint16, countNum uint16) {
	cnt.Lock()
	defer cnt.Unlock()
	cnt.countNum = 0
	cnt.epochNum++
	epochNum = cnt.epochNum
	countNum = cnt.countNum
	return
}
