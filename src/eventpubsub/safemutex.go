package eventpubsub

import (
	"sync"
)

type SafeMutex struct {
	m      sync.Mutex
	locked bool
}

func (sm *SafeMutex) Lock() {
	sm.m.Lock()
	sm.locked = true
}

func (sm *SafeMutex) Unlock() {
	sm.locked = false
	sm.m.Unlock()
}

func (sm *SafeMutex) IsLocked() bool {
	return sm.locked
}
