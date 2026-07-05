package safety

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// resetFsyncSeams restores the production fsync implementations after a test
// overrides them.
func resetFsyncSeams(t *testing.T) {
	t.Helper()
	origFile, origDir := fsyncFile, fsyncDir
	t.Cleanup(func() {
		fsyncFile = origFile
		fsyncDir = origDir
	})
}

func TestFileHaltStore_SaveRoundTripsThroughFsync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "halt-state.json")
	store := NewFileHaltStore(path)

	want := HaltState{Engaged: true, Reason: "durable engage", Source: SourceAuto, AckRequired: true}
	require.NoError(t, store.Save(want))

	got, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, want, got)

	// No stray temp files left behind after a successful save.
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestFileHaltStore_TempFsyncFailureReturnsErrorAndCleansUp(t *testing.T) {
	resetFsyncSeams(t)
	fsyncFile = func(f *os.File) error { return errors.New("disk on fire") }

	dir := t.TempDir()
	store := NewFileHaltStore(filepath.Join(dir, "halt-state.json"))

	err := store.Save(HaltState{Engaged: true, Reason: "won't stick", Source: SourceManual})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fsync temp")

	// The failed temp file must not be left behind, and no state file appears.
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	require.Empty(t, entries, "a failed fsync must not leave a temp or state file behind")
}

func TestFileHaltStore_DirFsyncFailureReturnsError(t *testing.T) {
	resetFsyncSeams(t)
	fsyncDir = func(d *os.File) error { return errors.New("directory flush failed") }

	dir := t.TempDir()
	store := NewFileHaltStore(filepath.Join(dir, "halt-state.json"))

	err := store.Save(HaltState{Engaged: true, Reason: "engage", Source: SourceAuto, AckRequired: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fsync directory")
}

func TestSyncDir_MissingDirectorySurfacesError(t *testing.T) {
	require.Error(t, syncDir(filepath.Join(t.TempDir(), "does-not-exist")))
}
