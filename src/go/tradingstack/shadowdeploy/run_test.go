package shadowdeploy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// TestRunShadow_PaperAndMarginRefused: the Simulation-only guard fires before
// anything is loaded or persisted -- a nil db proves nothing was touched, and
// the fake store records zero persist calls.
func TestRunShadow_PaperAndMarginRefused(t *testing.T) {
	window := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	for _, mode := range []models.Mode{models.ModePaper, models.ModeMargin} {
		store := NewFakeShadowStore()

		result, err := RunShadow(mode, nil, store, uuid.New(), window, window.AddDate(0, 0, 7), time.Now().UTC())

		require.Error(t, err, "mode %s must be refused", mode)
		assert.ErrorIs(t, err, ErrNotSimulation)
		assert.Nil(t, result)
		assert.Equal(t, 0, store.PersistCalls, "a refused run must persist nothing")
	}
}

// TestRunShadow_InvalidModeRefused: the zero value and garbage modes are also
// refused -- only Simulation, explicitly, may run.
func TestRunShadow_InvalidModeRefused(t *testing.T) {
	store := NewFakeShadowStore()
	window := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err := RunShadow(models.Mode(""), nil, store, uuid.New(), window, window.AddDate(0, 0, 7), time.Now().UTC())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotSimulation)
	assert.Equal(t, 0, store.PersistCalls)
}

// TestDivergencesFromReport_FlattensBothKinds: the report detail becomes
// persistable rows with the right kinds, scores, and run id.
func TestDivergencesFromReport_FlattensBothKinds(t *testing.T) {
	runID := uuid.New()
	report := DivergenceReport{
		ShadowOnly: []DivergentTicker{
			{Ticker: "AMZN", ActiveScore: floatPtr(0.47), ShadowScore: floatPtr(0.60), ScanResultID: uuid.New()},
		},
		ActiveOnly: []DivergentTicker{
			{Ticker: "AMD", ActiveScore: floatPtr(0.70), ShadowScore: floatPtr(0.30), ScanResultID: uuid.New()},
		},
	}

	rows := DivergencesFromReport(runID, report)

	require.Len(t, rows, 2)
	assert.Equal(t, KindShadowOnly, rows[0].Kind)
	assert.Equal(t, "AMZN", rows[0].Ticker)
	assert.Equal(t, KindActiveOnly, rows[1].Kind)
	assert.Equal(t, "AMD", rows[1].Ticker)
	for _, row := range rows {
		assert.Equal(t, runID, row.ShadowRunID)
		assert.NotNil(t, row.ActiveScore)
		assert.NotNil(t, row.ShadowScore)
	}
}

// TestFakeShadowStore_RejectsInvalidKind mirrors the production kind
// vocabulary in the in-memory fake.
func TestFakeShadowStore_RejectsInvalidKind(t *testing.T) {
	store := NewFakeShadowStore()

	err := store.PersistRun(ShadowRun{}, []ShadowDivergence{{Ticker: "AAPL", Kind: "sideways"}})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDivergenceKind)
}

// TestFakeShadowStore_RoundTrip: persisted runs read back by id with their
// divergences ordered by ticker, and list newest-first.
func TestFakeShadowStore_RoundTrip(t *testing.T) {
	store := NewFakeShadowStore()

	older := ShadowRun{ID: uuid.New(), CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	newer := ShadowRun{ID: uuid.New(), CreatedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)}

	require.NoError(t, store.PersistRun(older, []ShadowDivergence{
		{Ticker: "ZZZ", Kind: KindActiveOnly},
		{Ticker: "AAA", Kind: KindShadowOnly},
	}))
	require.NoError(t, store.PersistRun(newer, nil))

	run, divergences, err := store.FetchRun(older.ID)
	require.NoError(t, err)
	assert.Equal(t, older.ID, run.ID)
	require.Len(t, divergences, 2)
	assert.Equal(t, "AAA", divergences[0].Ticker)
	assert.Equal(t, "ZZZ", divergences[1].Ticker)
	for _, d := range divergences {
		assert.Equal(t, older.ID, d.ShadowRunID)
	}

	runs, err := store.ListRuns()
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, newer.ID, runs[0].ID, "list is newest first")

	_, _, err = store.FetchRun(uuid.New())
	assert.ErrorIs(t, err, ErrRunNotFound)
}
