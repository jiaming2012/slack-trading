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

// Save writes state to the configured path atomically and durably: the temp
// file is fsynced before the rename into place, and the parent directory is
// fsynced after the rename. Without those flushes a crash or power loss just
// after Save returns could roll the persisted halt state back to its previous
// value (or leave an empty file on some filesystems) — for an engage that
// means a halted server silently rebooting clear. Any flush failure surfaces
// as a save error rather than being swallowed.
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

	// Flush the new state to stable storage BEFORE the rename: rename is
	// atomic with respect to the namespace, but not with respect to the data
	// blocks — an unsynced temp file renamed into place can surface as empty
	// after a power loss.
	if err := fsyncFile(tmp); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("FileHaltStore.Save: fsync temp: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("FileHaltStore.Save: close temp: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("FileHaltStore.Save: rename into place: %w", err)
	}

	// Flush the directory entry AFTER the rename so the rename itself is
	// durable: without this, a crash can roll the path back to the previous
	// state file even though Save returned success.
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("FileHaltStore.Save: fsync directory %s: %w", dir, err)
	}

	return nil
}

// fsyncFile and fsyncDir are seams so tests can exercise the flush-failure
// paths deterministically; production always uses the real File.Sync.
var (
	fsyncFile = func(f *os.File) error { return f.Sync() }
	fsyncDir  = func(d *os.File) error { return d.Sync() }
)

// syncDir fsyncs a directory so a just-renamed entry inside it is durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return fsyncDir(d)
}
