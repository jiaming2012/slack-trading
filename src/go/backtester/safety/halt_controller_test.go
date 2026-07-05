package safety

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHaltController_DefaultsToClear(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)
	require.False(t, c.Status().Engaged)
	require.NoError(t, c.AllowOrder())
}

func TestHaltController_ManualEngageRejectsOrders(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.Engage("operator pulled the switch"))

	st := c.Status()
	require.True(t, st.Engaged)
	require.Equal(t, SourceManual, st.Source)
	require.False(t, st.AckRequired, "manual halt must not require acknowledgment")
	require.Equal(t, "operator pulled the switch", st.Reason)

	err = c.AllowOrder()
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrHalted))
}

func TestHaltController_ManualReleaseClears(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.Engage("test"))
	require.NoError(t, c.Release())
	require.False(t, c.Status().Engaged)
	require.NoError(t, c.AllowOrder())
}

// --- Cooldown protocol ---

func TestCooldown_ReleaseBlockedUntilAck(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.EngageAuto("rejection-rate guard"))

	st := c.Status()
	require.True(t, st.Engaged)
	require.Equal(t, SourceAuto, st.Source)
	require.True(t, st.AckRequired, "auto-halt must require acknowledgment")

	// Release without acknowledgment must be rejected and stay engaged.
	err = c.Release()
	require.ErrorIs(t, err, ErrAckRequired)
	require.True(t, c.Status().Engaged, "controller must remain engaged after a rejected release")
	require.ErrorIs(t, c.AllowOrder(), ErrHalted)
}

func TestCooldown_AcknowledgeThenReleaseResumes(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.EngageAuto("fill-deviation guard"))
	require.NoError(t, c.Acknowledge())
	require.False(t, c.Status().AckRequired)

	require.NoError(t, c.Release())
	require.False(t, c.Status().Engaged)
	require.NoError(t, c.AllowOrder(), "order submission resumes after ack + release")
}

func TestCooldown_NeverSelfClearsWhenAnomalySubsides(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.EngageAuto("trades-per-hour guard"))

	// The anomaly subsides — but nothing in the controller clears it. Only an
	// explicit acknowledge + release does. Re-evaluating a healthy metric must
	// not touch the halt; simulate by asserting state is unchanged.
	require.True(t, c.Status().Engaged)
	require.True(t, c.Status().AckRequired)
	require.ErrorIs(t, c.AllowOrder(), ErrHalted)

	// A subsequent (idempotent) auto-trip keeps it engaged/ack-required.
	require.NoError(t, c.EngageAuto("trades-per-hour guard"))
	require.True(t, c.Status().Engaged)
	require.True(t, c.Status().AckRequired)
}

func TestCooldown_AutoOverManualEscalatesToAckRequired(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.Engage("manual"))
	require.False(t, c.Status().AckRequired)

	// A guard trip during a manual halt escalates to auto + ack-required.
	require.NoError(t, c.EngageAuto("feed-staleness guard"))
	st := c.Status()
	require.Equal(t, SourceAuto, st.Source)
	require.True(t, st.AckRequired)
	require.ErrorIs(t, c.Release(), ErrAckRequired)
}

func TestCooldown_ManualEngageOverAutoPreservesAck(t *testing.T) {
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)

	require.NoError(t, c.EngageAuto("rejection-rate guard tripped"))

	// A manual engage over an active auto-halt must not launder it into a
	// freely-releasable manual halt: the ack requirement, the automatic source,
	// and the auto reason are all preserved.
	require.NoError(t, c.Engage("operator also pulled the switch"))

	st := c.Status()
	require.True(t, st.Engaged)
	require.Equal(t, SourceAuto, st.Source, "manual engage must not downgrade the source of an auto-halt")
	require.True(t, st.AckRequired, "manual engage must preserve the cooldown ack requirement of an auto-halt")
	require.Contains(t, st.Reason, "rejection-rate guard tripped", "the auto reason must be retained")
	require.Contains(t, st.Reason, "operator also pulled the switch", "the manual reason should be appended, not destroy the auto reason")

	// Release is still blocked until an explicit acknowledgment.
	require.ErrorIs(t, c.Release(), ErrAckRequired)
	require.True(t, c.Status().Engaged, "controller must remain engaged after a rejected release")

	// Only acknowledge + release clears it.
	require.NoError(t, c.Acknowledge())
	require.NoError(t, c.Release())
	require.False(t, c.Status().Engaged)
	require.NoError(t, c.AllowOrder())
}

func TestStatus_ReportsAckRequirementBySource(t *testing.T) {
	// Manual halt: no ack required.
	m, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)
	require.NoError(t, m.Engage("manual"))
	require.False(t, m.Status().AckRequired)

	// Auto halt: ack required.
	a, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)
	require.NoError(t, a.EngageAuto("guard"))
	require.True(t, a.Status().AckRequired)
}

// --- Persistence across restart ---

func TestPersistence_RestartWhileHaltedStaysHalted(t *testing.T) {
	store := NewMemoryHaltStore()

	c1, err := NewHaltController(store)
	require.NoError(t, err)
	require.NoError(t, c1.Engage("halt before restart"))

	// Simulate a process restart: a brand-new controller constructed from the
	// same store.
	c2, err := NewHaltController(store)
	require.NoError(t, err)

	st := c2.Status()
	require.True(t, st.Engaged, "a controller restored from a halted store must come back halted")
	require.Equal(t, "halt before restart", st.Reason)
	require.Equal(t, SourceManual, st.Source)
	require.ErrorIs(t, c2.AllowOrder(), ErrHalted)
}

func TestPersistence_RestartWhileClearStaysClear(t *testing.T) {
	store := NewMemoryHaltStore()

	c1, err := NewHaltController(store)
	require.NoError(t, err)
	require.NoError(t, c1.Engage("temp"))
	require.NoError(t, c1.Release())

	c2, err := NewHaltController(store)
	require.NoError(t, err)
	require.False(t, c2.Status().Engaged)
	require.NoError(t, c2.AllowOrder())
}

func TestPersistence_AutoHaltAckRequirementSurvivesRestart(t *testing.T) {
	store := NewMemoryHaltStore()

	c1, err := NewHaltController(store)
	require.NoError(t, err)
	require.NoError(t, c1.EngageAuto("guard tripped before restart"))

	c2, err := NewHaltController(store)
	require.NoError(t, err)
	require.True(t, c2.Status().Engaged)
	require.True(t, c2.Status().AckRequired, "the cooldown requirement must survive a restart")
	require.ErrorIs(t, c2.Release(), ErrAckRequired)
}

// --- File-backed store, including restart-across-instances ---

func TestFileHaltStore_MissingFileDefaultsToClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "halt-state.json")
	store := NewFileHaltStore(path)

	c, err := NewHaltController(store)
	require.NoError(t, err)
	require.False(t, c.Status().Engaged)
}

func TestFileHaltStore_RestartWhileHaltedStaysHalted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "halt-state.json")

	c1, err := NewHaltController(NewFileHaltStore(path))
	require.NoError(t, err)
	require.NoError(t, c1.Engage("persisted to disk"))

	// New store instance over the same path == a process restart.
	c2, err := NewHaltController(NewFileHaltStore(path))
	require.NoError(t, err)
	require.True(t, c2.Status().Engaged)
	require.Equal(t, "persisted to disk", c2.Status().Reason)
}

func TestFileHaltStore_EmptyFileDefaultsToClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "halt-state.json")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	c, err := NewHaltController(NewFileHaltStore(path))
	require.NoError(t, err)
	require.False(t, c.Status().Engaged)
}

// TestFileHaltStore_Exists backs the startup "store is empty" warning: a wiped
// or first-run state file must be detectable so the operator is loudly told the
// server is booting with no persisted halt state.
func TestFileHaltStore_Exists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "halt-state.json")
	store := NewFileHaltStore(path)

	// Missing file: not present.
	require.False(t, store.Exists(), "a missing state file must report not-exists")

	// Empty file (e.g. truncated by a wipe): treated as not present.
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))
	require.False(t, store.Exists(), "an empty state file must report not-exists")

	// After a real save the file exists and is non-empty.
	c, err := NewHaltController(store)
	require.NoError(t, err)
	require.NoError(t, c.Engage("persist something"))
	require.True(t, store.Exists(), "a non-empty state file must report exists")
}
