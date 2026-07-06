package shadowdeploy

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decisionFor builds a scored decision for divergence tests.
func decisionFor(ticker string, score float64, selected bool) Decision {
	return Decision{
		ScanResultID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("divergence-test-"+ticker)),
		Ticker:       ticker,
		Regime:       "trending",
		Admitted:     true,
		Scored:       true,
		Score:        score,
		Selected:     selected,
	}
}

// TestCompareDecisions_FiftyPercentFixture pins the spec fixture: active
// selects {A, B, C}, shadow selects {B, C, D} -> 1 shadow-only (D), 1
// active-only (A), 2 both, divergence 50% (2 divergent over a union of 4).
func TestCompareDecisions_FiftyPercentFixture(t *testing.T) {
	active := []Decision{
		decisionFor("A", 0.9, true),
		decisionFor("B", 0.8, true),
		decisionFor("C", 0.7, true),
		decisionFor("D", 0.2, false),
	}
	shadow := []Decision{
		decisionFor("A", 0.3, false),
		decisionFor("B", 0.85, true),
		decisionFor("C", 0.75, true),
		decisionFor("D", 0.95, true),
	}

	report := CompareDecisions(active, shadow)

	assert.Equal(t, 4, report.TotalObservations)
	assert.Equal(t, 3, report.SelectedActive)
	assert.Equal(t, 3, report.SelectedShadow)
	assert.Equal(t, 2, report.SelectedBoth)

	require.Len(t, report.ShadowOnly, 1)
	assert.Equal(t, "D", report.ShadowOnly[0].Ticker)
	require.NotNil(t, report.ShadowOnly[0].ActiveScore)
	assert.InDelta(t, 0.2, *report.ShadowOnly[0].ActiveScore, 1e-9)
	require.NotNil(t, report.ShadowOnly[0].ShadowScore)
	assert.InDelta(t, 0.95, *report.ShadowOnly[0].ShadowScore, 1e-9)

	require.Len(t, report.ActiveOnly, 1)
	assert.Equal(t, "A", report.ActiveOnly[0].Ticker)
	require.NotNil(t, report.ActiveOnly[0].ActiveScore)
	assert.InDelta(t, 0.9, *report.ActiveOnly[0].ActiveScore, 1e-9)
	require.NotNil(t, report.ActiveOnly[0].ShadowScore)
	assert.InDelta(t, 0.3, *report.ActiveOnly[0].ShadowScore, 1e-9)

	assert.InDelta(t, 50.0, report.DivergencePct, 1e-9)
}

// TestCompareDecisions_IdenticalSelectionsZeroDivergence: same selections on
// both sides -> 0% with empty divergence lists, even when scores differ.
func TestCompareDecisions_IdenticalSelectionsZeroDivergence(t *testing.T) {
	active := []Decision{
		decisionFor("A", 0.9, true),
		decisionFor("B", 0.8, true),
		decisionFor("C", 0.1, false),
	}
	shadow := []Decision{
		decisionFor("A", 0.5, true), // different score -- not divergence
		decisionFor("B", 0.6, true),
		decisionFor("C", 0.2, false),
	}

	report := CompareDecisions(active, shadow)

	assert.InDelta(t, 0.0, report.DivergencePct, 1e-9)
	assert.Empty(t, report.ShadowOnly)
	assert.Empty(t, report.ActiveOnly)
	assert.Equal(t, 2, report.SelectedBoth)
}

// TestCompareDecisions_EmptySelectionsNoDivisionByZero: neither side selects
// anything -> 0% via the max(1, |union|) guard, returning normally.
func TestCompareDecisions_EmptySelectionsNoDivisionByZero(t *testing.T) {
	active := []Decision{decisionFor("A", 0.1, false)}
	shadow := []Decision{decisionFor("A", 0.2, false)}

	report := CompareDecisions(active, shadow)

	assert.InDelta(t, 0.0, report.DivergencePct, 1e-9)
	assert.Equal(t, 0, report.SelectedActive)
	assert.Equal(t, 0, report.SelectedShadow)
	assert.Equal(t, 0, report.SelectedBoth)
	assert.Empty(t, report.ShadowOnly)
	assert.Empty(t, report.ActiveOnly)
}

// TestCompareDecisions_DeterministicOrdering: diverging tickers come back
// ascending regardless of input order, and repeated comparison is identical.
func TestCompareDecisions_DeterministicOrdering(t *testing.T) {
	active := []Decision{
		decisionFor("ZZZ", 0.9, true),
		decisionFor("AAA", 0.8, true),
		decisionFor("MMM", 0.7, true),
	}
	shadow := []Decision{
		decisionFor("ZZZ", 0.1, false),
		decisionFor("AAA", 0.1, false),
		decisionFor("MMM", 0.1, false),
	}

	first := CompareDecisions(active, shadow)
	second := CompareDecisions(active, shadow)
	assert.Equal(t, first, second)

	require.Len(t, first.ActiveOnly, 3)
	assert.Equal(t, "AAA", first.ActiveOnly[0].Ticker)
	assert.Equal(t, "MMM", first.ActiveOnly[1].Ticker)
	assert.Equal(t, "ZZZ", first.ActiveOnly[2].Ticker)
	assert.InDelta(t, 100.0, first.DivergencePct, 1e-9)
}

// TestCompareDecisions_NoModelSideCarriesNilScore: a ticker the active
// payload has no model for shows a nil active score in the divergence detail
// rather than a fabricated zero.
func TestCompareDecisions_NoModelSideCarriesNilScore(t *testing.T) {
	activeNoModel := Decision{
		ScanResultID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("divergence-test-D")),
		Ticker:       "D",
		Regime:       "unknown",
		NoModel:      true,
		Admitted:     true,
	}
	active := []Decision{activeNoModel}
	shadow := []Decision{decisionFor("D", 0.95, true)}

	report := CompareDecisions(active, shadow)

	require.Len(t, report.ShadowOnly, 1)
	assert.Nil(t, report.ShadowOnly[0].ActiveScore, "an unscored (no_model) side carries a nil score")
	require.NotNil(t, report.ShadowOnly[0].ShadowScore)
}
