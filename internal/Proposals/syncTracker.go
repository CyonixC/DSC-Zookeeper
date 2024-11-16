package proposals

import "sync"

// This file contains related definitions for the "syncTracker", which is the structure
// used to keep track of syncing new leader proposals.

type SyncTracker struct {
	sync.RWMutex
	active  bool
	zxidCap uint32
	prop    ZabMessage
}

func (s *SyncTracker) newSync(zxid uint32, p ZabMessage) {
	s.Lock()
	defer s.Unlock()
	s.active = true
	s.zxidCap = zxid
	s.prop = p
}

func (s *SyncTracker) deactivate() {
	s.RLock()
	defer s.RUnlock()
	s.active = false
}

func (s *SyncTracker) readVals() (bool, uint32) {
	s.RLock()
	defer s.RUnlock()
	return s.active, s.zxidCap
}

func (s *SyncTracker) getStoredProposal() ZabMessage {
	s.RLock()
	defer s.RUnlock()
	return s.prop
}
