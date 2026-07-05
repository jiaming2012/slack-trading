package safety

import "sync"

// MemoryHaltStore is an in-memory HaltStateStore for tests and for wiring guards
// without touching the filesystem. It is safe for concurrent use.
type MemoryHaltStore struct {
	mu     sync.Mutex
	state  HaltState
	loaded bool
}

// NewMemoryHaltStore returns an empty in-memory store (Load yields a clear
// state until the first Save).
func NewMemoryHaltStore() *MemoryHaltStore {
	return &MemoryHaltStore{}
}

// Load returns the last saved state, or a clear state if nothing was saved yet.
func (s *MemoryHaltStore) Load() (HaltState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		return HaltState{}, nil
	}
	return s.state, nil
}

// Save stores state in memory.
func (s *MemoryHaltStore) Save(state HaltState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.loaded = true
	return nil
}
