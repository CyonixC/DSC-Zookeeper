package proposals

// This file contains related definitions for the ACK counter, a map from proposal ZXIDs to ints
// which is used to track ACK counts for proposals sent out by the coordinator.
// When the coordinator sends out a proposal, a new entry is created in the counter, which is
// incremented with each ACK that it receives. This counter is locked to ensure thread safety.

import "sync"

type AckCounter struct {
	sync.Mutex
	ackTracker map[uint32]int
}

func (ackCnt *AckCounter) increment(zxid uint32) int {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	count, ok := ackCnt.ackTracker[zxid]
	if !ok {
		// Entry does not exist, just return 0
		return 0
	}
	ackCnt.ackTracker[zxid] = count + 1
	return count + 1
}

func (ackCnt *AckCounter) remove(zxid uint32) {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	delete(ackCnt.ackTracker, zxid)
}

// Performs similarly to the increment() function, but atomically removes the entry if the count
// exceeds `maxCount`.
func (ackCnt *AckCounter) incrementOrRemove(zxid uint32, maxCount int) bool {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	count, ok := ackCnt.ackTracker[zxid]
	if !ok {
		return false
	}
	if count+1 > maxCount {
		ackCnt.ackTracker[zxid] = 0
		return true
	}

	ackCnt.ackTracker[zxid] = count + 1
	return false
}

// Create new counter in the map
func (ackCnt *AckCounter) storeNew(zxid uint32) {
	ackCnt.Lock()
	defer ackCnt.Unlock()
	ackCnt.ackTracker[zxid] = 1
}
