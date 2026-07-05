package safety

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileHaltStore is the default HaltStateStore: it persists HaltState as JSON at
// a configured path. It performs an atomic write (temp file + rename) so a crash
// mid-write cannot leave a truncated, unparseable state file. A missing file is
// treated as "no state persisted yet" and yields a clear HaltState.
type FileHaltStore struct {
	mu   sync.Mutex
	path string
}

// NewFileHaltStore constructs a file-backed store writing to path. The parent
// directory is created on first Save if it does not exist. The path is
// typically derived from configuration/env at startup.
func NewFileHaltStore(path string) *FileHaltStore {
	return &FileHaltStore{path: path}
}

// Load reads and parses the persisted state. A missing file returns a clear
// HaltState with no error (fresh start). A present-but-unparseable file returns
// an error rather than silently defaulting to clear — a corrupt halt file is a
// safety signal, not something to paper over.
func (s *FileHaltStore) Load() (HaltState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return HaltState{}, nil
		}
		return HaltState{}, fmt.Errorf("FileHaltStore.Load: read %s: %w", s.path, err)
	}

	if len(data) == 0 {
		return HaltState{}, nil
	}

	var state HaltState
	if err := json.Unmarshal(data, &state); err != nil {
		return HaltState{}, fmt.Errorf("FileHaltStore.Load: parse %s: %w", s.path, err)
	}

	return state, nil
}

// Exists reports whether a non-empty persisted state file is present at the
// store's path. A false result at startup means the halt state is being
// bootstrapped from nothing — first run, or the state file was wiped. Because a
// wiped halt file silently boots a halted server back to clear, callers SHOULD
// log loudly when this returns false. The default path deliberately lives
// outside .cache/ (a wipe-by-convention directory) for exactly this reason.
func (s *FileHaltStore) Exists() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// Save writes state to the configured path atomically.
func (s *FileHaltStore) Save(state HaltState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("FileHaltStore.Save: marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("FileHaltStore.Save: mkdir %s: %w", dir, err)
		}
	}

	tmp, err := os.CreateTemp(dir, ".halt-state-*.tmp")
	if err != nil {
		return fmt.Errorf("FileHaltStore.Save: create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("FileHaltStore.Save: write temp: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("FileHaltStore.Save: close temp: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("FileHaltStore.Save: rename into place: %w", err)
	}

	return nil
}
